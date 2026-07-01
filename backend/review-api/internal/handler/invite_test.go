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
	h := NewInviteHandler(&fakeAccepter{}, zap.NewNop())
	r := setupInviteRouter(h, "")
	rec := postAcceptInvite(r, `{"repo_full_name":"owner/repo"}`)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}

func TestAcceptInvite_Success_204(t *testing.T) {
	accepter := &fakeAccepter{}
	h := NewInviteHandler(accepter, zap.NewNop())
	r := setupInviteRouter(h, "user-uuid-1")

	rec := postAcceptInvite(r, `{"repo_full_name":"owner/repo"}`)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d: %s", rec.Code, rec.Body.String())
	}
	if accepter.lastOwner != "owner" || accepter.lastRepo != "repo" {
		t.Fatalf("expected owner=owner repo=repo, got owner=%q repo=%q", accepter.lastOwner, accepter.lastRepo)
	}
}

func TestAcceptInvite_MissingRepoFullName_400(t *testing.T) {
	h := NewInviteHandler(&fakeAccepter{}, zap.NewNop())
	r := setupInviteRouter(h, "user-uuid-1")
	rec := postAcceptInvite(r, `{"repo_full_name":""}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAcceptInvite_MalformedRepoFullName_400(t *testing.T) {
	h := NewInviteHandler(&fakeAccepter{}, zap.NewNop())
	r := setupInviteRouter(h, "user-uuid-1")
	rec := postAcceptInvite(r, `{"repo_full_name":"no-slash"}`)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestAcceptInvite_AccepterError_502(t *testing.T) {
	h := NewInviteHandler(&fakeAccepter{err: errors.New("gitflame: no pending invitation")}, zap.NewNop())
	r := setupInviteRouter(h, "user-uuid-1")
	rec := postAcceptInvite(r, `{"repo_full_name":"owner/repo"}`)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("expected 502, got %d", rec.Code)
	}
}
