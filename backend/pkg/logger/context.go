package logger

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

type contextKey string

const loggerKey contextKey = "logger"

// InjectLogger attaches request-scoped fields to the logger in context.
func InjectLogger(base *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			enriched := base
			if id := CorrelationIDFromContext(r.Context()); id != "" {
				enriched = enriched.With(zap.String("correlation_id", id))
			}
			if traceID, spanID := TraceFromContext(r.Context()); traceID != "" {
				enriched = enriched.With(
					zap.String("trace_id", traceID),
					zap.String("span_id", spanID),
				)
			}
			ctx := context.WithValue(r.Context(), loggerKey, enriched)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithLogger returns a context carrying the given logger.
func WithLogger(ctx context.Context, log *zap.Logger) context.Context {
	return context.WithValue(ctx, loggerKey, log)
}

// FromContext returns the request logger from context, or a no-op logger if absent.
func FromContext(ctx context.Context) *zap.Logger {
	if log := fromContext(ctx); log != nil {
		return log
	}
	return zap.NewNop()
}

// FromContextOr returns the request logger from context, or fallback if absent.
//
// Prefer this over FromContext anywhere a real fallback logger exists:
// FromContext never returns nil, so `log := FromContext(ctx); if log == nil {
// log = fallback }` silently discards logs instead of using the fallback.
func FromContextOr(ctx context.Context, fallback *zap.Logger) *zap.Logger {
	if log := fromContext(ctx); log != nil {
		return log
	}
	return fallback
}

func fromContext(ctx context.Context) *zap.Logger {
	log, _ := ctx.Value(loggerKey).(*zap.Logger)
	return log
}
