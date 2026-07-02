package middleware

import (
	"context"
	"net/http"

	"go.uber.org/zap"
)

const LoggerKey contextKey = "logger"

// Logger injects the base logger into the request context.
func Logger(base *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			id := CorrelationIDFromContext(ctx)
			
			enrichedLogger := base
			if id != "" {
				enrichedLogger = base.With(zap.String("correlation_id", id))
			}

			ctx = context.WithValue(r.Context(), LoggerKey, enrichedLogger)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// WithLogger returns a context carrying the given logger.
func WithLogger(ctx context.Context, logger *zap.Logger) context.Context {
	return context.WithValue(ctx, LoggerKey, logger)
}

// LoggerFromContext returns the request logger, enriched with correlation_id when present.
func LoggerFromContext(ctx context.Context) *zap.Logger {
	log, _ := ctx.Value(LoggerKey).(*zap.Logger)
	if log == nil {
		log = zap.NewNop()
	}

	return log
}
