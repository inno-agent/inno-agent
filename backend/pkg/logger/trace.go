package logger

import (
	"context"
	"net/http"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
)

// PropagateHeaders forwards correlation ID and OpenTelemetry trace context.
func PropagateHeaders(ctx context.Context, req *http.Request) {
	SetCorrelationIDHeader(ctx, req)
	tracing.Propagate(ctx, req)
}

// TraceFromContext returns trace_id and span_id from the active OpenTelemetry span.
func TraceFromContext(ctx context.Context) (traceID, spanID string) {
	return tracing.TraceIDs(ctx)
}

// TraceFields returns zap fields for trace context in logs.
func TraceFields(ctx context.Context) []zap.Field {
	traceID, spanID := TraceFromContext(ctx)
	if traceID == "" {
		return nil
	}
	return []zap.Field{
		zap.String("trace_id", traceID),
		zap.String("span_id", spanID),
	}
}
