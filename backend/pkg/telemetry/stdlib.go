package telemetry

import (
	"net/http"
	"time"
)

type stdStatusWriter struct {
	http.ResponseWriter
	status int
}

func (w *stdStatusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

// Flush delegates to the wrapped ResponseWriter so SSE / streaming handlers
// can type-assert http.Flusher. Without this, embedding the http.ResponseWriter
// interface hides the underlying Flush method.
func (w *stdStatusWriter) Flush() {
	if f, ok := w.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

// StdMiddleware records HTTP metrics for net/http mux handlers.
func StdMiddleware(serviceName string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/metrics" {
			next.ServeHTTP(w, r)
			return
		}

		start := time.Now()
		trackInFlight(serviceName, 1)
		defer trackInFlight(serviceName, -1)

		sw := &stdStatusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		observe(serviceName, r.Method, r.URL.Path, sw.status, time.Since(start))
	})
}
