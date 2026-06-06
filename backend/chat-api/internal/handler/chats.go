package handler

import (
    "net/http"
    "strconv"

    "go.uber.org/zap"

    "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
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

    // TODO: replace with userID from JWT claims via auth middleware
    userID := r.URL.Query().Get("user_id")
    if userID == "" {
        h.logger.Warn("missing user_id", zap.String("function", "List"))
        writeError(w, http.StatusBadRequest, "user_id is required")
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
