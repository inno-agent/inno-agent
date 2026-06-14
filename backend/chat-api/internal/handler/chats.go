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

// ChatHandler handles HTTP requests for chat listing.
type ChatHandler struct {
	service domain.ChatService
	logger  *zap.Logger
}

// NewChatHandler creates a ChatHandler with the given service and logger.
func NewChatHandler(service domain.ChatService, logger *zap.Logger) *ChatHandler {
	return &ChatHandler{service: service, logger: logger}
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
		h.logger.Error("failed to list chats", zap.Error(err))
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
		if err.Error() == "delete chat: soft delete chat: chat not found or already deleted" {
			writeError(w, http.StatusNotFound, "chat not found")
			return
		}
		h.logger.Error("failed to delete chat", zap.Error(err))
		writeError(w, http.StatusInternalServerError, "failed to delete chat")
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status":  "deleted",
		"chat_id": chatID.String(),
	})
}
