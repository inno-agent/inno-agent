package handler

import (
    "net/http"
    "strconv"

    "go.uber.org/zap"

    // "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain/dtos"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain/services"
)

type ChatHandler struct {
    service services.Service
    logger  *zap.Logger
}

func NewChatHandler(service services.Service, logger *zap.Logger) *ChatHandler {
    return &ChatHandler{service: service, logger: logger}
}

func (h *ChatHandler) List(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    userID := r.URL.Query().Get("user_id")
    if userID == "" {
        h.logger.Error("missing user_id", zap.String("function", "List"))
        writeError(w, http.StatusBadRequest, "user_id is required")
        return
    }

    limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
    offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
    if limit <= 0 || limit > 50 {
        limit = 10
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