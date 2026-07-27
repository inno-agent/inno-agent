package tracing

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.37.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/inno-agent/inno-agent/backend/pkg/logger"
)

// Init configures OpenTelemetry trace export (Jaeger via OTLP).
// If OTEL_EXPORTER_OTLP_ENDPOINT is unset, tracing stays disabled (noop).
func Init(ctx context.Context, service string) (func(context.Context) error, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	endpoint = strings.TrimPrefix(endpoint, "http://")
	endpoint = strings.TrimPrefix(endpoint, "https://")
	if endpoint == "" {
		otel.SetTextMapPropagator(propagation.TraceContext{})
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(
		ctx,
		otlptracegrpc.WithEndpoint(endpoint),
		otlptracegrpc.WithTLSCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	res, err := resource.Merge(
		resource.Default(),
		resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(service),
		),
	)
	if err != nil {
		_ = exporter.Shutdown(ctx)
		return nil, err
	}

	// Parse trace sampling ratio from env (default 1.0 for always-on).
	samplingRatio := 1.0
	if ratioStr := strings.TrimSpace(os.Getenv("OTEL_TRACES_SAMPLER_RATIO")); ratioStr != "" {
		if ratio, err := strconv.ParseFloat(ratioStr, 64); err == nil && ratio >= 0.0 && ratio <= 1.0 {
			samplingRatio = ratio
		}
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(samplingRatio))),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	// Set error handler to log export errors to stderr.
	otel.SetErrorHandler(otel.ErrorHandlerFunc(func(err error) {
		fmt.Fprintf(os.Stderr, "tracing export error: %v\n", err)
	}))

	// Log startup info to stderr.
	fmt.Fprintf(os.Stderr, "tracing: exporting to %s for %s\n", endpoint, service)

	return tp.Shutdown, nil
}

// Setup configures tracing and returns a cleanup function for graceful shutdown.
// Call defer cleanup() from main after Setup succeeds.
func Setup(ctx context.Context, service string) (cleanup func(), err error) {
	shutdown, err := Init(ctx, service)
	if err != nil {
		return nil, err
	}
	return func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), ShutdownTimeout)
		defer cancel()
		_ = shutdown(shutdownCtx)
	}, nil
}

// HTTPMiddleware wraps a handler with an OpenTelemetry span per request.
func HTTPMiddleware(service string, next http.Handler) http.Handler {
	return otelhttp.NewHandler(
		next, service,
		otelhttp.WithSpanNameFormatter(func(operation string, r *http.Request) string {
			return r.Method + " " + r.URL.Path
		}),
	)
}

// ChiMiddleware returns chi-compatible OpenTelemetry HTTP middleware.
func ChiMiddleware(service string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return HTTPMiddleware(service, next)
	}
}

// GinMiddleware returns gin middleware that records OpenTelemetry spans.
func GinMiddleware(service string) gin.HandlerFunc {
	return otelgin.Middleware(service)
}

// Propagate injects trace context into outbound HTTP request headers.
func Propagate(ctx context.Context, req *http.Request) {
	otel.GetTextMapPropagator().Inject(ctx, propagation.HeaderCarrier(req.Header))
}

// PropagateOutbound forwards correlation ID and trace context on outbound HTTP calls.
func PropagateOutbound(ctx context.Context, req *http.Request) {
	logger.PropagateHeaders(ctx, req)
	Propagate(ctx, req)
}

// TraceIDs returns trace_id and span_id from the active span, if any.
func TraceIDs(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}

// StartSpan begins a manual span (e.g. Kafka message processing).
func StartSpan(ctx context.Context, tracerName, spanName string) (context.Context, trace.Span) {
	return otel.Tracer(tracerName).Start(ctx, spanName)
}

// ShutdownTimeout is the default graceful shutdown window for the tracer provider.
const ShutdownTimeout = 5 * time.Second

// KafkaHeader is a key/value pair for Kafka message propagation.
type KafkaHeader struct {
	Key   string
	Value string
}

// KafkaHeadersFromContext builds Kafka headers carrying trace and correlation context.
func KafkaHeadersFromContext(ctx context.Context) []KafkaHeader {
	carrier := propagation.MapCarrier{}
	otel.GetTextMapPropagator().Inject(ctx, carrier)
	if id := logger.CorrelationIDFromContext(ctx); id != "" {
		carrier[logger.Header] = id
	}
	headers := make([]KafkaHeader, 0, len(carrier))
	for k, v := range carrier {
		headers = append(headers, KafkaHeader{Key: k, Value: v})
	}
	return headers
}

// ContextFromKafkaHeaders restores trace and correlation context from Kafka headers.
func ContextFromKafkaHeaders(ctx context.Context, headers []KafkaHeader) context.Context {
	carrier := propagation.MapCarrier{}
	for _, h := range headers {
		carrier[h.Key] = h.Value
	}
	ctx = otel.GetTextMapPropagator().Extract(ctx, carrier)
	return logger.ContextWithCorrelationID(ctx, carrier[logger.Header])
}
