package telemetry

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

// The metrics wrappers embed the http.ResponseWriter interface, which hides the
// underlying Flush method. SSE / streaming handlers type-assert http.Flusher, so
// the wrappers MUST re-expose Flush or streaming breaks with a 500.
var (
	_ http.Flusher = (*stdStatusWriter)(nil)
	_ http.Flusher = (*statusWriter)(nil)
)

func TestWrappersFlushToUnderlyingWriter(t *testing.T) {
	rec := httptest.NewRecorder() // implements http.Flusher

	std := &stdStatusWriter{ResponseWriter: rec}
	if _, ok := interface{}(std).(http.Flusher); !ok {
		t.Fatal("stdStatusWriter does not implement http.Flusher")
	}
	std.Flush()
	if !rec.Flushed {
		t.Fatal("stdStatusWriter.Flush did not reach underlying writer")
	}

	rec2 := httptest.NewRecorder()
	chi := &statusWriter{ResponseWriter: rec2}
	chi.Flush()
	if !rec2.Flushed {
		t.Fatal("statusWriter.Flush did not reach underlying writer")
	}
}
