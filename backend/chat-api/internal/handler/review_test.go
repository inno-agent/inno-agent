package handler

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

type mockReviewService struct {
	domain.ReviewService
	reviewFn func(ctx context.Context, prID string, diff string) (string, error)
}

func (m *mockReviewService) ReviewPR(ctx context.Context, prID string, diff string) (string, error) {
	return m.reviewFn(ctx, prID, diff)
}

func newReviewRouter(h *ReviewHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/review", h.Review)
	return r
}

func postReview(r *chi.Mux, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/review", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestReview_InvalidBody(t *testing.T) {
	h := NewReviewHandler(&mockReviewService{}, zap.NewNop())
	r := newReviewRouter(h)

	rec := postReview(r, "not-json")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestReview_MissingPRID(t *testing.T) {
	h := NewReviewHandler(&mockReviewService{}, zap.NewNop())
	r := newReviewRouter(h)

	rec := postReview(r, `{}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestReview_Success(t *testing.T) {
	svc := &mockReviewService{
		reviewFn: func(_ context.Context, prID string, diff string) (string, error) {
			if prID != "123" {
				t.Fatalf("expected pr_id 123, got %q", prID)
			}
			if diff != "diff --git a/main.go" {
				t.Fatalf("expected diff payload, got %q", diff)
			}
			return "# Summary\nLooks good.", nil
		},
	}
	h := NewReviewHandler(svc, zap.NewNop())
	r := newReviewRouter(h)

	rec := postReview(r, `{"pr_id":"123","diff":"diff --git a/main.go"}`)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "# Summary") {
		t.Fatalf("expected review markdown in body, got %s", rec.Body.String())
	}
}

func TestReview_DiffUnavailable(t *testing.T) {
	svc := &mockReviewService{
		reviewFn: func(_ context.Context, _ string, _ string) (string, error) {
			return "", domain.ErrDiffUnavailable
		},
	}
	h := NewReviewHandler(svc, zap.NewNop())
	r := newReviewRouter(h)

	rec := postReview(r, `{"pr_id":"123"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestReview_ServiceError(t *testing.T) {
	svc := &mockReviewService{
		reviewFn: func(_ context.Context, _ string, _ string) (string, error) {
			return "", errors.New("upstream failure")
		},
	}
	h := NewReviewHandler(svc, zap.NewNop())
	r := newReviewRouter(h)

	rec := postReview(r, `{"pr_id":"123","diff":"diff content"}`)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
