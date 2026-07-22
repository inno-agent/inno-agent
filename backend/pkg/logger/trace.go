package logger

import (
	"context"
	"net/http"

	"go.opentelemetry.io/otel/trace"
)

// PropagateHeaders forwards the correlation ID to outbound HTTP requests.
func PropagateHeaders(ctx context.Context, req *http.Request) {
	SetCorrelationIDHeader(ctx, req)
}

// TraceFromContext returns trace_id and span_id from the active OpenTelemetry span.
func TraceFromContext(ctx context.Context) (traceID, spanID string) {
	sc := trace.SpanFromContext(ctx).SpanContext()
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}
