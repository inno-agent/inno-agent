package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/installation"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/middleware"
)

// InviteAccepter confirms the bot account's pending collaborator invitation on a repo.
type InviteAccepter interface {
	AcceptInvite(ctx context.Context, owner, repo string) error
}

// InstallationLookup resolves the caller's own linked gitflame_username.
type InstallationLookup interface {
	GetGitFlameUsername(ctx context.Context, userID string) (string, error)
}

type acceptInviteRequest struct {
	RepoName string `json:"repo_name"`
}

// InviteHandler handles requests to accept pending GitFlame collaborator invitations.
type InviteHandler struct {
	lookup   InstallationLookup
	accepter InviteAccepter
	logger   *zap.Logger
}

// NewInviteHandler creates an InviteHandler.
func NewInviteHandler(lookup InstallationLookup, accepter InviteAccepter, logger *zap.Logger) *InviteHandler {
	return &InviteHandler{lookup: lookup, accepter: accepter, logger: logger}
}

// AcceptInvite handles POST /api/v1/invitations/accept.
// The repo owner is always the caller's own linked GitFlame account — never taken
// from the request body — so a caller can only accept invites on their own repos.
func (h *InviteHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	repo := strings.TrimSpace(req.RepoName)
	if repo == "" || strings.Contains(repo, "/") {
		writeError(w, http.StatusBadRequest, "repo_name must be a repo name, not owner/repo")
		return
	}

	owner, err := h.lookup.GetGitFlameUsername(ctx, userID)
	if err != nil {
		if errors.Is(err, installation.ErrNotLinked) {
			writeError(w, http.StatusConflict, "gitflame_account_not_linked")
			return
		}
		h.logger.Error("lookup gitflame username", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if err := h.accepter.AcceptInvite(ctx, owner, repo); err != nil {
		h.logger.Error("accept invite", zap.String("owner", owner), zap.String("repo", repo), zap.Error(err))
		writeError(w, http.StatusBadGateway, "accept_failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
