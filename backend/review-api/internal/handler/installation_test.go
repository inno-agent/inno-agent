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

// fakeEncryptor returns deterministic ciphertext/nonce for testing.
type fakeEncryptor struct{}

func (fakeEncryptor) Encrypt(plaintext []byte) ([]byte, []byte, error) {
	return append([]byte("ct:"), plaintext...), []byte("nonce-12-bytes"), nil
}

// fakeStore records the last upsert and can simulate ownership conflicts.
type fakeStore struct {
	err           error
	gotUserID     string
	gotUsername   string
	gotCiphertext []byte
}

func (f *fakeStore) Upsert(_ context.Context, gitflameUsername, userID string, ciphertext, _ []byte) error {
	if f.err != nil {
		return f.err
	}
	f.gotUsername = gitflameUsername
	f.gotUserID = userID
	f.gotCiphertext = ciphertext
	return nil
}

// withUserID injects a user_id into the request context like the Auth middleware would.
func withUserID(userID string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), middleware.UserIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func newInstallRouter(h *InstallationHandler, userID string) *chi.Mux {
	r := chi.NewRouter()
	if userID != "" {
		r.Use(withUserID(userID))
	}
	r.Post("/installations", h.Create)
	return r
}

func postInstall(r *chi.Mux, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/installations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestInstallation_NoUserID_401(t *testing.T) {
	store := &fakeStore{}
	h := NewInstallationHandler(store, fakeEncryptor{}, zap.NewNop())
	// No user_id injected (simulating missing/failed auth).
	r := newInstallRouter(h, "")

	rec := postInstall(r, `{"gitflame_username":"alice","refresh_token":"rt-123"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestInstallation_Success_204(t *testing.T) {
	store := &fakeStore{}
	h := NewInstallationHandler(store, fakeEncryptor{}, zap.NewNop())
	r := newInstallRouter(h, "user-uuid-1")

	rec := postInstall(r, `{"gitflame_username":"alice","refresh_token":"rt-123"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if store.gotUserID != "user-uuid-1" {
		t.Fatalf("expected user_id user-uuid-1, got %q", store.gotUserID)
	}
	if store.gotUsername != "alice" {
		t.Fatalf("expected username alice, got %q", store.gotUsername)
	}
	if string(store.gotCiphertext) != "ct:rt-123" {
		t.Fatalf("expected encrypted refresh token, got %q", store.gotCiphertext)
	}
}

func TestInstallation_OwnedByAnother_409(t *testing.T) {
	store := &fakeStore{err: installation.ErrOwnedByAnother}
	h := NewInstallationHandler(store, fakeEncryptor{}, zap.NewNop())
	r := newInstallRouter(h, "user-uuid-2")

	rec := postInstall(r, `{"gitflame_username":"alice","refresh_token":"rt-456"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestInstallation_MissingFields_400(t *testing.T) {
	store := &fakeStore{}
	h := NewInstallationHandler(store, fakeEncryptor{}, zap.NewNop())
	r := newInstallRouter(h, "user-uuid-3")

	rec := postInstall(r, `{"gitflame_username":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}
