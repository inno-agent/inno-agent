package tracing

import (
	"context"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"

	"github.com/inno-agent/inno-agent/backend/pkg/logger"
)

func TestKafkaHeaderPropagation(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	ctx := logger.ContextWithCorrelationID(context.Background(), "corr-123")
	ctx, span := StartSpan(ctx, "review-webhook", "publish")
	defer span.End()

	headers := KafkaHeadersFromContext(ctx)
	if len(headers) == 0 {
		t.Fatal("expected kafka headers")
	}

	got := map[string]string{}
	for _, h := range headers {
		got[h.Key] = h.Value
	}
	if got[logger.Header] != "corr-123" {
		t.Fatalf("correlation header = %q", got[logger.Header])
	}
	if got["traceparent"] == "" {
		t.Fatal("expected traceparent header")
	}

	restored := ContextFromKafkaHeaders(context.Background(), headers)
	if logger.CorrelationIDFromContext(restored) != "corr-123" {
		t.Fatalf("correlation = %q", logger.CorrelationIDFromContext(restored))
	}
	traceID, _ := TraceIDs(restored)
	if traceID == "" {
		t.Fatal("expected trace_id in restored context")
	}
}
