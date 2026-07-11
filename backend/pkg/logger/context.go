package logger

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

type contextKey string

const loggerKey contextKey = "logger"

// InjectLogger attaches a correlation-enriched logger to the request context.
func InjectLogger(base *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			enriched := base
			if id := CorrelationIDFromContext(r.Context()); id != "" {
				enriched = base.With(zap.String("correlation_id", id))
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
	log, _ := ctx.Value(loggerKey).(*zap.Logger)
	if log == nil {
		return zap.NewNop()
	}
	return log
}
