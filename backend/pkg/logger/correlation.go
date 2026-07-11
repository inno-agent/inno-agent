package logger

import (
	"context"
	"net/http"

	"github.com/google/uuid"
)

// Header is the HTTP header used for request correlation IDs.
const Header = "X-Correlation-ID"

const correlationIDKey contextKey = "correlation_id"

// CorrelationID ensures every request has a correlation ID in context and response headers.
func CorrelationID(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(Header)
		if id == "" {
			id = uuid.NewString()
		}
		w.Header().Set(Header, id)
		ctx := context.WithValue(r.Context(), correlationIDKey, id)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// CorrelationIDFromContext extracts correlation_id from request context.
func CorrelationIDFromContext(ctx context.Context) string {
	v, _ := ctx.Value(correlationIDKey).(string)
	return v
}

// SetCorrelationIDHeader propagates the correlation ID to outbound HTTP requests.
func SetCorrelationIDHeader(ctx context.Context, req *http.Request) {
	if id := CorrelationIDFromContext(ctx); id != "" {
		req.Header.Set(Header, id)
	}
}
