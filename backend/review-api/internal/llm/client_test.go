package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/middleware"
)

func TestOrchestratorClient_Chat_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/v1/chat" {
			t.Fatalf("expected /v1/chat, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"answer": "Looks good!"}`))
	}))
	defer srv.Close()

	client := NewOrchestratorClient(srv.URL)
	messages := []domain.LLMMessage{
		{Role: "system", Content: "You are a reviewer"},
		{Role: "user", Content: "Review this code"},
	}

	result, err := client.Chat(context.Background(), messages, "qwen2.5-coder:1.5b")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "Looks good!" {
		t.Fatalf("expected 'Looks good!', got %q", result)
	}
}

func TestOrchestratorClient_Chat_4xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"error": "bad model"}`))
	}))
	defer srv.Close()

	client := NewOrchestratorClient(srv.URL)
	_, err := client.Chat(context.Background(), []domain.LLMMessage{}, "")

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOrchestratorClient_Chat_5xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Write([]byte(`{"error": "unavailable"}`))
	}))
	defer srv.Close()

	client := NewOrchestratorClient(srv.URL)
	_, err := client.Chat(context.Background(), []domain.LLMMessage{}, "")

	if err == nil {
		t.Fatal("expected error")
	}
}

func TestOrchestratorClient_Chat_WithToken(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"answer": "ok"}`))
	}))
	defer srv.Close()

	client := NewOrchestratorClient(srv.URL)
	ctx := context.WithValue(context.Background(), middleware.TokenKey, "my-jwt-token")

	_, err := client.Chat(ctx, []domain.LLMMessage{}, "model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "Bearer my-jwt-token" {
		t.Fatalf("expected 'Bearer my-jwt-token', got %q", gotAuth)
	}
}

func TestOrchestratorClient_Chat_EmptyModel(t *testing.T) {
	var body map[string]interface{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req map[string]interface{}
		json.NewDecoder(r.Body).Decode(&req)
		body = req
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"answer": "ok"}`))
	}))
	defer srv.Close()

	client := NewOrchestratorClient(srv.URL)
	_, err := client.Chat(context.Background(), []domain.LLMMessage{}, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, exists := body["model_name"]; exists {
		t.Fatalf("model_name should not be present when empty, got %v", body)
	}
}
