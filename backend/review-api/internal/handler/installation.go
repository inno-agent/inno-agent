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
}

type installationRequest struct {
	GitFlameUsername string `json:"gitflame_username"`
}

// InstallationHandler handles onboarding requests linking a GitFlame username
// to the caller's user_id.
type InstallationHandler struct {
	store  InstallationStore
	logger *zap.Logger
}

// NewInstallationHandler creates an InstallationHandler.
func NewInstallationHandler(store InstallationStore, logger *zap.Logger) *InstallationHandler {
	return &InstallationHandler{store: store, logger: logger}
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
