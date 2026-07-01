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

	"github.com/inno-agent/inno-agent/backend/review-api/internal/installation"
)

type fakeAccepter struct {
	err       error
	lastOwner string
	lastRepo  string
}

func (f *fakeAccepter) AcceptInvite(_ context.Context, owner, repo string) error {
	if f.err != nil {
		return f.err
	}
	f.lastOwner = owner
	f.lastRepo = repo
	return nil
}

type fakeLookup struct {
	username string
	err      error
}

func (f *fakeLookup) GetGitFlameUsername(_ context.Context, _ string) (string, error) {
	if f.err != nil {
		return "", f.err
	}
	return f.username, nil
}

func setupInviteRouter(h *InviteHandler, userID string) *chi.Mux {
	r := chi.NewRouter()
	if userID != "" {
		r.Use(withUserID(userID))
	}
	r.Post("/api/v1/invitations/accept", h.AcceptInvite)
	return r
}

func postAcceptInvite(r *chi.Mux, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/invitations/accept", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestAcceptInvite_NoUserID_401(t *testing.T) {
	h := NewInviteHandler(&fakeLookup{username: "owner"}, &fakeAccepter{}, zap.NewNop())
	r := setupInviteRouter(h, "")
	rec := postAcceptInvite(r, `{"repo_name":"repo"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAcceptInvite_Success_204(t *testing.T) {
	accepter := &fakeAccepter{}
	h := NewInviteHandler(&fakeLookup{username: "owner"}, accepter, zap.NewNop())
	r := setupInviteRouter(h, "user-uuid-1")

	rec := postAcceptInvite(r, `{"repo_name":"repo"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if accepter.lastOwner != "owner" || accepter.lastRepo != "repo" {
		t.Fatalf("expected owner=owner repo=repo, got owner=%q repo=%q", accepter.lastOwner, accepter.lastRepo)
	}
}

func TestAcceptInvite_OwnerAlwaysFromLookup_NotRequestBody(t *testing.T) {
	// Even if a client tries to sneak an owner/repo path into repo_name, the
	// resolved owner must come from the caller's own linked account, never
	// from the request body — otherwise any authenticated caller could accept
	// invites on someone else's repo.
	accepter := &fakeAccepter{}
	h := NewInviteHandler(&fakeLookup{username: "victim"}, accepter, zap.NewNop())
	r := setupInviteRouter(h, "attacker-uuid")

	rec := postAcceptInvite(r, `{"repo_name":"attacker-owned-repo/../victim-repo"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for slash in repo_name, got %d: %s", rec.Code, rec.Body.String())
	}
}

func TestAcceptInvite_MissingRepoName_400(t *testing.T) {
	h := NewInviteHandler(&fakeLookup{username: "owner"}, &fakeAccepter{}, zap.NewNop())
	r := setupInviteRouter(h, "user-uuid-1")
	rec := postAcceptInvite(r, `{"repo_name":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAcceptInvite_RepoNameWithSlash_400(t *testing.T) {
	h := NewInviteHandler(&fakeLookup{username: "owner"}, &fakeAccepter{}, zap.NewNop())
	r := setupInviteRouter(h, "user-uuid-1")
	rec := postAcceptInvite(r, `{"repo_name":"owner/repo"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAcceptInvite_NotLinked_409(t *testing.T) {
	h := NewInviteHandler(&fakeLookup{err: installation.ErrNotLinked}, &fakeAccepter{}, zap.NewNop())
	r := setupInviteRouter(h, "user-uuid-1")
	rec := postAcceptInvite(r, `{"repo_name":"repo"}`)
	if rec.Code != http.StatusConflict {
		t.Fatalf("expected 409, got %d", rec.Code)
	}
}

func TestAcceptInvite_AccepterError_502(t *testing.T) {
	h := NewInviteHandler(&fakeLookup{username: "owner"}, &fakeAccepter{err: errors.New("gitflame: no pending invitation")}, zap.NewNop())
	r := setupInviteRouter(h, "user-uuid-1")
	rec := postAcceptInvite(r, `{"repo_name":"repo"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}
