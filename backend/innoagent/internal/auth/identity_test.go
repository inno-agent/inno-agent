package auth

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"

	"innoagent/internal/correlation"
)

func TestIdentityClient_Validate_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if r.URL.Path != "/identity/v1/validate" {
			t.Fatalf("expected /identity/v1/validate, got %s", r.URL.Path)
		}
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user_id": "user-456"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	userID, err := client.Validate(context.Background(), "valid-token")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if userID != "user-456" {
		t.Fatalf("expected 'user-456', got %q", userID)
	}
}

func TestIdentityClient_Validate_Invalid(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error": "invalid token"}`))
	}))
	defer srv.Close()

	client := NewClient(srv.URL)
	_, err := client.Validate(context.Background(), "bad-token")

	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, ErrUnauthorized) {
		t.Fatalf("expected ErrUnauthorized, got %v", err)
	}
}

func TestIdentityClient_Validate_Down(t *testing.T) {
	client := NewClient("http://localhost:99999")
	_, err := client.Validate(context.Background(), "token")

	if err == nil {
		t.Fatal("expected error for connection refused")
	}
}

func TestIdentityClient_Validate_CorrelationID(t *testing.T) {
	var gotCorrelationID string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotCorrelationID = r.Header.Get("X-Correlation-ID")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"user_id": "user-123"}`))
	}))
	defer srv.Close()

	// Use middleware to set correlation ID in context
	var ctx context.Context
	middleware := correlation.Middleware(zap.NewNop())
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Correlation-ID", "test-corr-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	client := NewClient(srv.URL)
	_, err := client.Validate(ctx, "token")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotCorrelationID != "test-corr-id" {
		t.Fatalf("expected 'test-corr-id', got %q", gotCorrelationID)
	}
}
