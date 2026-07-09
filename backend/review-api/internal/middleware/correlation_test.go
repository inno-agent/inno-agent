package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCorrelationID_GeneratesUUID(t *testing.T) {
	var gotID string
	handler := CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotID == "" {
		t.Fatal("expected correlation ID to be generated")
	}
	respID := rec.Header().Get("X-Correlation-ID")
	if respID == "" {
		t.Fatal("expected X-Correlation-ID header")
	}
	if gotID != respID {
		t.Fatalf("context ID %q != header ID %q", gotID, respID)
	}
}

func TestCorrelationID_ForwardsExisting(t *testing.T) {
	var gotID string
	handler := CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = CorrelationIDFromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Correlation-ID", "my-custom-id")
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if gotID != "my-custom-id" {
		t.Fatalf("expected 'my-custom-id', got %q", gotID)
	}
	if rec.Header().Get("X-Correlation-ID") != "my-custom-id" {
		t.Fatal("expected header to match")
	}
}

func TestCorrelationID_SetsResponseHeader(t *testing.T) {
	handler := CorrelationID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Header().Get("X-Correlation-ID") == "" {
		t.Fatal("expected X-Correlation-ID header to be set")
	}
}

func TestCorrelationIDFromContext_Empty(t *testing.T) {
	id := CorrelationIDFromContext(context.Background())
	if id != "" {
		t.Fatalf("expected empty, got %q", id)
	}
}
