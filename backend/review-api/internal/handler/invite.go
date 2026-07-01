package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"

	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-api/internal/middleware"
)

// InviteAccepter confirms the bot account's pending collaborator invitation on a repo.
type InviteAccepter interface {
	AcceptInvite(ctx context.Context, owner, repo string) error
}

type acceptInviteRequest struct {
	RepoFullName string `json:"repo_full_name"`
}

// InviteHandler handles requests to accept pending GitFlame collaborator invitations.
type InviteHandler struct {
	accepter InviteAccepter
	logger   *zap.Logger
}

// NewInviteHandler creates an InviteHandler.
func NewInviteHandler(accepter InviteAccepter, logger *zap.Logger) *InviteHandler {
	return &InviteHandler{accepter: accepter, logger: logger}
}

// AcceptInvite handles POST /api/v1/invitations/accept.
func (h *InviteHandler) AcceptInvite(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	if middleware.UserIDFromContext(ctx) == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req acceptInviteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	owner, repo, ok := strings.Cut(strings.TrimSpace(req.RepoFullName), "/")
	if !ok || owner == "" || repo == "" {
		writeError(w, http.StatusBadRequest, "repo_full_name must be owner/repo")
		return
	}

	if err := h.accepter.AcceptInvite(ctx, owner, repo); err != nil {
		h.logger.Error("accept invite", zap.String("repo", req.RepoFullName), zap.Error(err))
		writeError(w, http.StatusBadGateway, "accept_failed")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
