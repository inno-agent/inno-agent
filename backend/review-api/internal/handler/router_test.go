package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
)

func TestRegisterRoutes_HealthEndpoint(t *testing.T) {
	r := chi.NewRouter()
	handler := &ReviewHandler{}
	RegisterRoutes(r, handler, "")

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
}

func TestRegisterRoutes_CORS(t *testing.T) {
	r := chi.NewRouter()
	handler := &ReviewHandler{}
	RegisterRoutes(r, handler, "")

	req := httptest.NewRequest(http.MethodOptions, "/api/v1/review", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Allow-Origin") != "*" {
		t.Fatal("expected CORS header")
	}
}
