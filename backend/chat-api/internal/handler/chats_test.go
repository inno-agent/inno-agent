package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

// mockChatService implements domain.ChatService for chat handler tests.
type mockChatService struct {
	domain.ChatService // embed for unimplemented methods
	listChats  func(ctx context.Context, userID string, limit, offset int) ([]domain.ChatItem, int, error)
	getHistory func(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]domain.MessageDTO, int, error)
}

func (m *mockChatService) ListChats(ctx context.Context, userID string, limit, offset int) ([]domain.ChatItem, int, error) {
	return m.listChats(ctx, userID, limit, offset)
}

func (m *mockChatService) GetHistory(ctx context.Context, userID string, chatID uuid.UUID, limit, offset int) ([]domain.MessageDTO, int, error) {
	return m.getHistory(ctx, userID, chatID, limit, offset)
}

func newTestChatHandler(svc domain.ChatService) *ChatHandler {
	return NewChatHandler(svc, zap.NewNop())
}

func TestChatList_MissingUserID(t *testing.T) {
	h := newTestChatHandler(&mockChatService{})

	r := chi.NewRouter()
	r.Get("/chats", h.List)

	req := httptest.NewRequest(http.MethodGet, "/chats", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}

	var body map[string]string
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body["error"] == "" {
		t.Fatal("expected non-empty error field")
	}
}

func TestChatList_EmptyList(t *testing.T) {
	svc := &mockChatService{
		listChats: func(_ context.Context, _ string, _, _ int) ([]domain.ChatItem, int, error) {
			return []domain.ChatItem{}, 0, nil
		},
	}
	h := newTestChatHandler(svc)

	r := chi.NewRouter()
	r.Get("/chats", h.List)

	req := httptest.NewRequest(http.MethodGet, "/chats?user_id=u1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Chats []domain.ChatItem `json:"chats"`
		Total int               `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Total != 0 {
		t.Fatalf("expected total 0, got %d", body.Total)
	}
	if body.Chats == nil || len(body.Chats) != 0 {
		t.Fatalf("expected empty chats slice, got %v", body.Chats)
	}
}

func TestChatList_TwoChats(t *testing.T) {
	now := time.Now().Truncate(time.Second)
	items := []domain.ChatItem{
		{ID: uuid.New(), Title: "Chat 1", LastMessage: "hello", UpdatedAt: now},
		{ID: uuid.New(), Title: "Chat 2", LastMessage: "world", UpdatedAt: now},
	}
	svc := &mockChatService{
		listChats: func(_ context.Context, _ string, _, _ int) ([]domain.ChatItem, int, error) {
			return items, len(items), nil
		},
	}
	h := newTestChatHandler(svc)

	r := chi.NewRouter()
	r.Get("/chats", h.List)

	req := httptest.NewRequest(http.MethodGet, "/chats?user_id=u1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var body struct {
		Chats []domain.ChatItem `json:"chats"`
		Total int               `json:"total"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatalf("decode error: %v", err)
	}
	if body.Total != 2 {
		t.Fatalf("expected total 2, got %d", body.Total)
	}
	if len(body.Chats) != 2 {
		t.Fatalf("expected 2 chats, got %d", len(body.Chats))
	}
	if body.Chats[0].ID != items[0].ID {
		t.Fatalf("first chat ID mismatch: got %s", body.Chats[0].ID)
	}
	if body.Chats[1].ID != items[1].ID {
		t.Fatalf("second chat ID mismatch: got %s", body.Chats[1].ID)
	}
}

func TestChatList_ServiceError(t *testing.T) {
	svc := &mockChatService{
		listChats: func(_ context.Context, _ string, _, _ int) ([]domain.ChatItem, int, error) {
			return nil, 0, errors.New("db is down")
		},
	}
	h := newTestChatHandler(svc)

	r := chi.NewRouter()
	r.Get("/chats", h.List)

	req := httptest.NewRequest(http.MethodGet, "/chats?user_id=u1", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
}

func TestChatList_InvalidLimitFallsToDefault(t *testing.T) {
	var capturedLimit int
	svc := &mockChatService{
		listChats: func(_ context.Context, _ string, limit, _ int) ([]domain.ChatItem, int, error) {
			capturedLimit = limit
			return []domain.ChatItem{}, 0, nil
		},
	}
	h := newTestChatHandler(svc)

	r := chi.NewRouter()
	r.Get("/chats", h.List)

	req := httptest.NewRequest(http.MethodGet, "/chats?user_id=u1&limit=abc", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedLimit != 20 {
		t.Fatalf("expected default limit 20, got %d", capturedLimit)
	}
}

func TestChatList_LimitTooLargeFallsToDefault(t *testing.T) {
	var capturedLimit int
	svc := &mockChatService{
		listChats: func(_ context.Context, _ string, limit, _ int) ([]domain.ChatItem, int, error) {
			capturedLimit = limit
			return []domain.ChatItem{}, 0, nil
		},
	}
	h := newTestChatHandler(svc)

	r := chi.NewRouter()
	r.Get("/chats", h.List)

	req := httptest.NewRequest(http.MethodGet, "/chats?user_id=u1&limit=200", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if capturedLimit != 20 {
		t.Fatalf("expected limit clamped to 20, got %d", capturedLimit)
	}
}
