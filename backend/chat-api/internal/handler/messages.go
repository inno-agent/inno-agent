package handler

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
	"github.com/inno-agent/inno-agent/backend/chat-api/internal/middleware"
)

// MessageHandler handles HTTP requests for chat message history.
type MessageHandler struct {
	service domain.ChatService
	logger  *zap.Logger
}

// NewMessageHandler creates a MessageHandler with the given service and logger.
func NewMessageHandler(service domain.ChatService, logger *zap.Logger) *MessageHandler {
	return &MessageHandler{service: service, logger: logger}
}

// ListByChat returns paginated message history for the given chat.
func (h *MessageHandler) ListByChat(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	chatID, err := uuid.Parse(chi.URLParam(r, "chat_id"))
	if err != nil {
		h.logger.Error("invalid chat_id", zap.Error(err))
		writeError(w, http.StatusBadRequest, "invalid chat_id")
		return
	}

	userID := middleware.UserIDFromContext(ctx)
	if userID == "" {
		writeError(w, http.StatusUnauthorized, "unauthorized")
		return
	}

	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
	if limit <= 0 || limit > 100 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	messages, total, err := h.service.GetHistory(ctx, userID, chatID, limit, offset)
	if err != nil {
		h.logger.Error("failed to get history", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to get history")
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"chat_id":  chatID.String(),
		"messages": messages,
		"total":    total,
	})
}
