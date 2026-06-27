package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

const correlationIDHeader = "X-Correlation-ID"

const CorrelationIDKey contextKey = "correlation_id"

type responseWriter struct {
	http.ResponseWriter
	status int
	bytes  int
}

func (w *responseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *responseWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	n, err := w.ResponseWriter.Write(b)
	w.bytes += n
	return n, err
}

func (w *responseWriter) Flush() {
	if flusher, ok := w.ResponseWriter.(http.Flusher); ok {
		flusher.Flush()
	}
}

// CorrelationID ensures every request has a correlation ID in context and response headers.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(correlationIDHeader)
		if id == "" {
			id = uuid.NewString()
		}

		w.Header().Set(correlationIDHeader, id)
		ctx := context.WithValue(r.Context(), CorrelationIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// RequestLogger writes one structured access log line per HTTP request.
func RequestLogger(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := &responseWriter{ResponseWriter: w}

			next.ServeHTTP(ww, r)

			status := ww.status
			if status == 0 {
				status = http.StatusOK
			}

			logger.Info("http_request",
				zap.String("correlation_id", CorrelationIDFromContext(r.Context())),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.String("query", r.URL.RawQuery),
				zap.Int("status", status),
				zap.Int("bytes", ww.bytes),
				zap.Duration("duration", time.Since(start)),
				zap.String("remote_addr", r.RemoteAddr),
				zap.String("user_agent", r.UserAgent()),
			)
		})
	}
}

// CorrelationIDFromContext extracts correlation_id from request context.
func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(CorrelationIDKey).(string)
	return v
}

// SetCorrelationIDHeader propagates the correlation ID to outbound HTTP requests.
func SetCorrelationIDHeader(ctx context.Context, req *http.Request) {
	if id := CorrelationIDFromContext(ctx); id != "" {
		req.Header.Set(correlationIDHeader, id)
	}
}
