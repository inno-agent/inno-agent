package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
	"github.com/inno-agent/inno-agent/backend/chat-api/internal/middleware"
	"github.com/inno-agent/inno-agent/backend/pkg/logger"
)

// ChatHandler handles HTTP requests for chat listing.
type ChatHandler struct {
	service domain.ChatService
}

// NewChatHandler creates a ChatHandler with the given service.
func NewChatHandler(service domain.ChatService) *ChatHandler {
	return &ChatHandler{service: service}
}

// List returns a paginated list of chats for the requesting user.
func (h *ChatHandler) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	if offset < 0 {
		offset = 0
	}

	chats, total, err := h.service.ListChats(ctx, userID, limit, offset)
	if err != nil {
		logger.FromContext(ctx).Error("failed to list chats", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to list chats")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"chats": chats,
		"total": total,
	})
}

func (h *ChatHandler) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}
	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat_id")
		return
	}
	if err := h.service.DeleteChat(ctx, userID, chatID); err != nil {
		if errors.Is(err, domain.ErrNotFound) {
			writeError(w, http.StatusNotFound, "chat not found")
			return
		}
		if errors.Is(err, domain.ErrAccessDenied) {
			writeError(w, http.StatusForbidden, "access denied")
			return
		}
		logger.FromContext(ctx).Error("failed to delete chat", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to delete chat")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "deleted",
		"chat_id": chatID.String(),
	})
}
