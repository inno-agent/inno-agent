package handler

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/installation"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/middleware"
)

type fakeStore struct {
	err                  error
	lastGitFlameUsername string
	lastUserID           string

	getUsername string
	getErr      error
}

func (f *fakeStore) Upsert(_ context.Context, gitflameUsername, userID string) error {
	if f.err != nil {
		return f.err
	}
	f.lastGitFlameUsername = gitflameUsername
	f.lastUserID = userID
	return nil
}

func (f *fakeStore) GetGitFlameUsername(_ context.Context, _ string) (string, error) {
	if f.getErr != nil {
		return "", f.getErr
	}
	return f.getUsername, nil
}

type fakeDelegationGranter struct {
	err error
}

func (f *fakeDelegationGranter) GrantDelegation(_ context.Context, _, _ string) error {
	return f.err
}

func withUserID(userID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func setupRouter(h *InstallationHandler, userID string) *chi.Mux {
	r := chi.NewRouter()
	if userID != "" {
		r.Use(withUserID(userID))
	}
	r.Post("/api/v1/installations", h.Create)
	r.Get("/api/v1/installations/me", h.Get)
	return r
}

func postInstall(r *chi.Mux, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/installations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func getInstall(r *chi.Mux) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/installations/me", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestInstallation_NoUserID_401(t *testing.T) {
	h := NewInstallationHandler(&fakeStore{}, &fakeDelegationGranter{}, "review-consumer", zap.NewNop())
	r := setupRouter(h, "")
	rec := postInstall(r, `{"gitflame_username":"alice"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestInstallation_Success_204(t *testing.T) {
	store := &fakeStore{}
	h := NewInstallationHandler(store, &fakeDelegationGranter{}, "review-consumer", zap.NewNop())
	r := setupRouter(h, "user-uuid-1")

	rec := postInstall(r, `{"gitflame_username":"alice"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.lastUserID != "user-uuid-1" {
		t.Fatalf("expected user_id user-uuid-1, got %q", store.lastUserID)
	}
	if store.lastGitFlameUsername != "alice" {
		t.Fatalf("expected username alice, got %q", store.lastGitFlameUsername)
	}
}

func TestInstallation_OwnedByAnother_409(t *testing.T) {
	h := NewInstallationHandler(&fakeStore{err: installation.ErrOwnedByAnother}, &fakeDelegationGranter{}, "review-consumer", zap.NewNop())
	r := setupRouter(h, "user-uuid-2")
	rec := postInstall(r, `{"gitflame_username":"alice"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestInstallation_MissingUsername_400(t *testing.T) {
	h := NewInstallationHandler(&fakeStore{}, &fakeDelegationGranter{}, "review-consumer", zap.NewNop())
	r := setupRouter(h, "user-uuid-3")
	rec := postInstall(r, `{"gitflame_username":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestInstallationGet_NoUserID_401(t *testing.T) {
	h := NewInstallationHandler(&fakeStore{}, &fakeDelegationGranter{}, "review-consumer", zap.NewNop())
	r := setupRouter(h, "")
	rec := getInstall(r)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestInstallationGet_Linked_200(t *testing.T) {
	h := NewInstallationHandler(&fakeStore{getUsername: "alice"}, &fakeDelegationGranter{}, "review-consumer", zap.NewNop())
	r := setupRouter(h, "user-uuid-1")
	rec := getInstall(r)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "alice") {
		t.Fatalf("expected body to contain alice, got %s", rec.Body.String())
	}
}

func TestInstallationGet_NotLinked_404(t *testing.T) {
	h := NewInstallationHandler(&fakeStore{getErr: installation.ErrNotLinked}, &fakeDelegationGranter{}, "review-consumer", zap.NewNop())
	r := setupRouter(h, "user-uuid-1")
	rec := getInstall(r)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
}
