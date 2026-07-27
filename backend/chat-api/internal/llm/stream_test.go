package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func sseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprint(w, body)
	}))
}

// An orchestrator error event must surface as a Stream error, not an empty
// silently-"done" stream.
func TestStream_SurfacesOrchestratorErrorEvent(t *testing.T) {
	srv := sseServer(t, "data: {\"error\":\"model exploded\"}\n\n")
	defer srv.Close()

	c := NewOrchestratorClient(srv.URL)
	ch, err := c.Stream(context.Background(), []Message{{Role: "user", Content: "hi"}}, "")
	if err == nil {
		t.Fatal("expected error from orchestrator error event, got nil")
	}
	if !strings.Contains(err.Error(), "model exploded") {
		t.Fatalf("error should carry the upstream message, got: %v", err)
	}
	if ch != nil {
		t.Fatal("channel should be nil on stream error")
	}
}

func TestStream_YieldsAnswerChunks(t *testing.T) {
	srv := sseServer(t, "data: {\"answer\":\"Hel\"}\n\ndata: {\"answer\":\"lo\"}\n\ndata: [DONE]\n\n")
	defer srv.Close()

	c := NewOrchestratorClient(srv.URL)
	ch, err := c.Stream(context.Background(), []Message{{Role: "user", Content: "hi"}}, "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	var got strings.Builder
	for chunk := range ch {
		got.WriteString(chunk)
	}
	if got.String() != "Hello" {
		t.Fatalf("assembled = %q, want %q", got.String(), "Hello")
	}
}
