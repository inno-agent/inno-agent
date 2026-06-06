package handler

import (
    "errors"
    "net/http"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/google/uuid"
    "go.uber.org/zap"

    "github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

// StreamHandler handles SSE streaming of LLM responses.
type StreamHandler struct {
    service domain.ChatService
    logger  *zap.Logger
}

// NewStreamHandler creates a StreamHandler with the given service and logger.
func NewStreamHandler(service domain.ChatService, logger *zap.Logger) *StreamHandler {
    return &StreamHandler{service: service, logger: logger}
}

// Stream sends a user message and streams LLM response chunks via SSE.
func (h *StreamHandler) Stream(w http.ResponseWriter, r *http.Request) {
    ctx := r.Context()

    chatIDParam := chi.URLParam(r, "chat_id")
    var chatID uuid.UUID
    if chatIDParam != "" && chatIDParam != "new" {
        var err error
        chatID, err = uuid.Parse(chatIDParam)
        if err != nil {
            h.logger.Error("invalid chat_id", zap.Error(err))
            writeError(w, http.StatusBadRequest, "invalid chat_id")
            return
        }
    }

    // TODO: replace with userID from JWT claims via auth middleware
    userID := r.URL.Query().Get("user_id")
    if userID == "" {
        h.logger.Error("missing user_id", zap.String("function", "Stream"))
        writeError(w, http.StatusBadRequest, "user_id is required")
        return
    }
    message := r.URL.Query().Get("message")
    if message == "" {
        h.logger.Error("missing message", zap.String("function", "Stream"))
        writeError(w, http.StatusBadRequest, "message is required")
        return
    }

    flusher, ok := w.(http.Flusher)
    if !ok {
        h.logger.Error("streaming not supported")
        writeError(w, http.StatusInternalServerError, "streaming not supported")
        return
    }

    w.Header().Set("Content-Type", "text/event-stream")
    w.Header().Set("Cache-Control", "no-cache")
    w.Header().Set("Connection", "keep-alive")
    w.Header().Set("X-Accel-Buffering", "no")

    writeSSEEvent(w, "status", map[string]string{"stage": "context_loading"})
    flusher.Flush()

    ch, resolvedChatID, err := h.service.Stream(ctx, userID, chatID, message)
    if err != nil {
        h.logger.Error("failed to start stream", zap.Error(err))
        switch {
        case errors.Is(err, domain.ErrAccessDenied):
            writeSSEEvent(w, "error", map[string]string{"code": "AUTH_FAILED", "message": "access denied"})
        case errors.Is(err, domain.ErrNotFound):
            writeSSEEvent(w, "error", map[string]string{"code": "CHAT_NOT_FOUND", "message": "chat not found"})
        default:
            writeSSEEvent(w, "error", map[string]string{"code": "INTERNAL_ERROR", "message": "internal error"})
        }
        flusher.Flush()
        return
    }

    writeSSEEvent(w, "status", map[string]string{"stage": "llm_processing", "chat_id": resolvedChatID.String()})
    flusher.Flush()

loop:
    for {
        select {
        case chunk, ok := <-ch:
            if !ok {
                break loop
            }
            writeSSEEvent(w, "chunk", map[string]string{"content": chunk})
            flusher.Flush()
        case <-ctx.Done():
            return
        }
    }

    writeSSEEvent(w, "done", map[string]interface{}{
        "status":      "completed",
        "finished_at": time.Now().UTC().Format(time.RFC3339),
    })
    flusher.Flush()
}
