package correlation

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"time"

	"go.uber.org/zap"
)

const Header = "X-Correlation-ID"

type contextKey string

const correlationIDKey contextKey = "correlation_id"

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

// Middleware attaches correlation_id to the request context and emits JSON access logs.
func Middleware(logger *zap.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			id := r.Header.Get(Header)
			if id == "" {
				id = newID()
			}

			ww := &responseWriter{ResponseWriter: w}
			ww.Header().Set(Header, id)

			ctx := context.WithValue(r.Context(), correlationIDKey, id)
			next.ServeHTTP(ww, r.WithContext(ctx))

			status := ww.status
			if status == 0 {
				status = http.StatusOK
			}

			logger.Info("http_request",
				zap.String("correlation_id", id),
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

func FromContext(ctx context.Context) string {
	id, _ := ctx.Value(correlationIDKey).(string)
	return id
}

func SetHeader(ctx context.Context, req *http.Request) {
	if id := FromContext(ctx); id != "" {
		req.Header.Set(Header, id)
	}
}

func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b[:])
}
