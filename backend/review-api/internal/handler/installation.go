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

// InstallationStore persists installation rows.
type InstallationStore interface {
	Upsert(ctx context.Context, gitflameUsername, userID string) error
	GetGitFlameUsername(ctx context.Context, userID string) (string, error)
}

// DelegationGranter creates delegation grants in identity on behalf of a user.
type DelegationGranter interface {
	GrantDelegation(ctx context.Context, userToken, clientID string) error
}

type installationRequest struct {
	GitFlameUsername string `json:"gitflame_username"`
}

// InstallationHandler handles onboarding requests linking a GitFlame username
// to the caller's user_id.
type InstallationHandler struct {
	store          InstallationStore
	grantor        DelegationGranter
	reviewClientID string
	logger         *zap.Logger
}

// NewInstallationHandler creates an InstallationHandler.
func NewInstallationHandler(store InstallationStore, grantor DelegationGranter, reviewClientID string, logger *zap.Logger) *InstallationHandler {
	return &InstallationHandler{
		store:          store,
		grantor:        grantor,
		reviewClientID: reviewClientID,
		logger:         logger,
	}
}

// Create handles POST /api/v1/installations.
func (h *InstallationHandler) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	var req installationRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	req.GitFlameUsername = strings.TrimSpace(req.GitFlameUsername)
	if req.GitFlameUsername == "" {
		writeError(w, http.StatusBadRequest, "gitflame_username is required")
		return
	}

	// Establish delegation grant before recording the installation.
	// review-consumer needs this grant to exchange tokens on the user's behalf.
	userToken := middleware.TokenFromContext(ctx)
	if err := h.grantor.GrantDelegation(ctx, userToken, h.reviewClientID); err != nil {
		h.logger.Error("create delegation grant", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if err := h.store.Upsert(ctx, req.GitFlameUsername, userID); err != nil {
		if errors.Is(err, installation.ErrOwnedByAnother) {
			writeError(w, http.StatusConflict, "username_taken")
			return
		}
		h.logger.Error("upsert installation", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

type installationResponse struct {
	GitFlameUsername string `json:"gitflame_username"`
}

// Get handles GET /api/v1/installations/me. It lets the frontend restore
// onboarding state after a page reload instead of re-asking for the username.
func (h *InstallationHandler) Get(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	username, err := h.store.GetGitFlameUsername(ctx, userID)
	if err != nil {
		if errors.Is(err, installation.ErrNotLinked) {
			writeError(w, http.StatusNotFound, "not_linked")
			return
		}
		h.logger.Error("get installation", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	writeJSON(w, http.StatusOK, installationResponse{GitFlameUsername: username})
}
