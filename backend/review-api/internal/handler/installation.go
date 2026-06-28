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

// Encryptor encrypts a plaintext into (ciphertext, nonce).
// *secretbox.SecretBox satisfies it.
type Encryptor interface {
	Encrypt(plaintext []byte) (ciphertext []byte, nonce []byte, err error)
}

// InstallationStore persists installation rows.
// *installation.Repository satisfies it.
type InstallationStore interface {
	Upsert(ctx context.Context, gitflameUsername, userID string, ciphertext, nonce []byte) error
}

type installationRequest struct {
	GitFlameUsername string `json:"gitflame_username"`
	RefreshToken     string `json:"refresh_token"`
}

// InstallationHandler handles onboarding requests linking a gitflame username
// to the caller's user_id along with their encrypted refresh token.
type InstallationHandler struct {
	store  InstallationStore
	enc    Encryptor
	logger *zap.Logger
}

// NewInstallationHandler creates an InstallationHandler.
func NewInstallationHandler(store InstallationStore, enc Encryptor, logger *zap.Logger) *InstallationHandler {
	return &InstallationHandler{store: store, enc: enc, logger: logger}
}

// Create handles POST /api/v1/installations. It is mounted behind the Auth
// middleware, so user_id is taken from the validated token.
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
	if req.RefreshToken == "" {
		writeError(w, http.StatusBadRequest, "refresh_token is required")
		return
	}

	ciphertext, nonce, err := h.enc.Encrypt([]byte(req.RefreshToken))
	if err != nil {
		h.logger.Error("encrypt refresh token", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "internal")
		return
	}

	if err := h.store.Upsert(ctx, req.GitFlameUsername, userID, ciphertext, nonce); err != nil {
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
