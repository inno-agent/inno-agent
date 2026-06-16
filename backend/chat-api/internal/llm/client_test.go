package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/middleware"
)

func TestChat_ForwardsBearerToken(t *testing.T) {
	var gotAuth string
	var gotPath string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answer":"hi"}`))
	}))
	defer srv.Close()

	c := NewOrchestratorClient(srv.URL)
	ctx := context.WithValue(context.Background(), middleware.TokenKey, "tok123")
	_, err := c.Chat(ctx, []Message{{Role: "user", Content: "x"}}, "")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotAuth != "Bearer tok123" {
		t.Fatalf("want forwarded bearer, got %q", gotAuth)
	}
	if gotPath != "/v1/chat" {
		t.Fatalf("want /v1/chat, got %q", gotPath)
	}
}
