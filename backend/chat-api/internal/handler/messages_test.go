package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
	"github.com/inno-agent/inno-agent/backend/chat-api/internal/middleware"
)

// mockMsgService implements domain.ChatService for message handler tests.
type mockMsgService struct {
	domain.ChatService
	getHistory func(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]domain.MessageDTO, int, error)
}

func (m *mockMsgService) GetHistory(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]domain.MessageDTO, int, error) {
	return m.getHistory(ctx, userID, chatID, limit, offset)
}

func newTestMessageHandler(svc domain.ChatService) *MessageHandler {
	return NewMessageHandler(svc)
}

func newMsgRouter(h *MessageHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/chats/{chat_id}/messages", h.ListByChat)
	return r
}

func getMessages(r *chi.Mux, chatID, userID string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/chats/"+chatID+"/messages", nil)
	if userID != "" {
		ctx := context.WithValue(req.Context(), middleware.UserIDKey, userID)
		req = req.WithContext(ctx)
	}
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestMessageListByChat_InvalidUUID(t *testing.T) {
	h := newTestMessageHandler(&mockMsgService{})
	r := newMsgRouter(h)

	rec := getMessages(r, "not-a-uuid", "u1")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid UUID, got %d", rec.Code)
	}
}

func TestMessageListByChat_NoAuth_Returns401(t *testing.T) {
	h := newTestMessageHandler(&mockMsgService{})
	r := newMsgRouter(h)

	chatID := uuid.New()
	rec := getMessages(r, chatID.String(), "")
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 for missing auth, got %d", rec.Code)
	}
}

func TestMessageListByChat_EmptyResult(t *testing.T) {
	svc := &mockMsgService{
		getHistory: func(_ context.Context, _ string, _ uuid.UUID, _, _ int) ([]domain.MessageDTO, int, error) {
			return []domain.MessageDTO{}, 0, nil
		},
	}
	h := newTestMessageHandler(svc)
	r := newMsgRouter(h)

	chatID := uuid.New()
	rec := getMessages(r, chatID.String(), "u1")
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body map[string]interface{}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["chat_id"] != chatID.String() {
		t.Fatalf("expected chat_id %s, got %v", chatID.String(), body["chat_id"])
	}
	msgs, ok := body["messages"].([]interface{})
	if !ok {
		t.Fatalf("messages field missing or wrong type: %v", body["messages"])
	}
	if len(msgs) != 0 {
		t.Fatalf("expected empty messages, got %d", len(msgs))
	}
	if total, ok := body["total"].(float64); !ok || total != 0 {
		t.Fatalf("expected total 0, got %v", body["total"])
	}
}

func TestMessageListByChat_ServiceError(t *testing.T) {
	svc := &mockMsgService{
		getHistory: func(_ context.Context, _ string, _ uuid.UUID, _, _ int) ([]domain.MessageDTO, int, error) {
			return nil, 0, errors.New("unexpected error")
		},
	}
	h := newTestMessageHandler(svc)
	r := newMsgRouter(h)

	chatID := uuid.New()
	rec := getMessages(r, chatID.String(), "u1")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
