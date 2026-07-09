package correlation

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.uber.org/zap"
)

func TestFromContext_Empty(t *testing.T) {
	id := FromContext(context.Background())
	if id != "" {
		t.Fatalf("expected empty, got %q", id)
	}
}

func TestFromContext_WithID(t *testing.T) {
	ctx := context.WithValue(context.Background(), correlationIDKey, "my-id")
	id := FromContext(ctx)
	if id != "my-id" {
		t.Fatalf("expected 'my-id', got %q", id)
	}
}

func TestSetHeader_WithID(t *testing.T) {
	ctx := context.WithValue(context.Background(), correlationIDKey, "corr-123")
	req := httptest.NewRequest(http.MethodGet, "/", nil)

	SetHeader(ctx, req)

	if req.Header.Get(Header) != "corr-123" {
		t.Fatalf("expected 'corr-123', got %q", req.Header.Get(Header))
	}
}

func TestSetHeader_Empty(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	SetHeader(context.Background(), req)

	if req.Header.Get(Header) != "" {
		t.Fatalf("expected empty header, got %q", req.Header.Get(Header))
	}
}

func TestMiddleware_GeneratesID(t *testing.T) {
	var gotID string
	handler := Middleware(zap.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotID == "" {
		t.Fatal("expected correlation ID to be generated")
	}
	if rec.Header().Get(Header) == "" {
		t.Fatal("expected X-Correlation-ID header")
	}
}

func TestMiddleware_ForwardsExisting(t *testing.T) {
	var gotID string
	handler := Middleware(zap.NewNop())(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotID = FromContext(r.Context())
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set(Header, "existing-id")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if gotID != "existing-id" {
		t.Fatalf("expected 'existing-id', got %q", gotID)
	}
}
