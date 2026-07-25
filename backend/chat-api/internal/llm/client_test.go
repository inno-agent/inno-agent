package llm

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/middleware"
	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
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

func TestChat_ForwardsTraceContext(t *testing.T) {
	tp := sdktrace.NewTracerProvider()
	defer func() { _ = tp.Shutdown(context.Background()) }()
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.TraceContext{})

	var gotTraceparent string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotTraceparent = r.Header.Get("traceparent")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"answer":"hi"}`))
	}))
	defer srv.Close()

	ctx, span := tracing.StartSpan(context.Background(), "chat-api", "llm.chat")
	defer span.End()

	c := NewOrchestratorClient(srv.URL)
	_, err := c.Chat(ctx, []Message{{Role: "user", Content: "x"}}, "")
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if gotTraceparent == "" {
		t.Fatal("expected traceparent header on outbound orchestrator request")
	}
}
