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
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
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
	return NewMessageHandler(svc, zap.NewNop())
}

func newMsgRouter(h *MessageHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/chats/{chat_id}/messages", h.ListByChat)
	return r
}

func TestMessageListByChat_InvalidUUID(t *testing.T) {
	h := newTestMessageHandler(&mockMsgService{})
	r := newMsgRouter(h)

	req := httptest.NewRequest(http.MethodGet, "/chats/not-a-uuid/messages?user_id=u1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid UUID, got %d", rec.Code)
	}
}

func TestMessageListByChat_MissingUserID(t *testing.T) {
	h := newTestMessageHandler(&mockMsgService{})
	r := newMsgRouter(h)

	chatID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/chats/"+chatID.String()+"/messages", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing user_id, got %d", rec.Code)
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
	req := httptest.NewRequest(http.MethodGet, "/chats/"+chatID.String()+"/messages?user_id=u1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

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
	req := httptest.NewRequest(http.MethodGet, "/chats/"+chatID.String()+"/messages?user_id=u1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}
