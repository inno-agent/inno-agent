package tracing

import (
	"context"
	"net/http"
	"os"
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
)

// Init configures OpenTelemetry trace export (Jaeger via OTLP).
// If OTEL_EXPORTER_OTLP_ENDPOINT is unset, tracing stays disabled (noop).
func Init(ctx context.Context, service string) (func(context.Context) error, error) {
	endpoint := strings.TrimSpace(os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"))
	if endpoint == "" {
		otel.SetTextMapPropagator(propagation.TraceContext{})
		return func(context.Context) error { return nil }, nil
	}

	exporter, err := otlptracegrpc.New(ctx,
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
		return nil, err
	}

	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

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
	return otelhttp.NewHandler(next, service)
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
