package handler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/domain"
)

type mockStreamService struct {
	domain.ChatService
	streamFn func(ctx context.Context, userID string, chatID uuid.UUID, message string) (<-chan string, uuid.UUID, error)
}

func (m *mockStreamService) Stream(ctx context.Context, userID string, chatID uuid.UUID, message string) (<-chan string, uuid.UUID, error) {
	return m.streamFn(ctx, userID, chatID, message)
}

func newStreamRouter(h *StreamHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/chats/{chat_id}/stream", h.Stream)
	return r
}

func newTestStreamHandler(svc domain.ChatService) *StreamHandler {
	return NewStreamHandler(svc, zap.NewNop())
}

func getStream(r *chi.Mux, chatID, userID, message string) *httptest.ResponseRecorder {
	url := "/chats/" + chatID + "/stream"
	sep := "?"
	if userID != "" {
		url += sep + "user_id=" + userID
		sep = "&"
	}
	if message != "" {
		url += sep + "message=" + message
	}
	req := httptest.NewRequest(http.MethodGet, url, nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func TestStream_MissingUserID(t *testing.T) {
	h := newTestStreamHandler(&mockStreamService{})
	r := newStreamRouter(h)

	rec := getStream(r, "new", "", "hello")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestStream_MissingMessage(t *testing.T) {
	h := newTestStreamHandler(&mockStreamService{})
	r := newStreamRouter(h)

	rec := getStream(r, "new", "u1", "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", rec.Code)
	}
}

func TestStream_NewChat_SendsChunks(t *testing.T) {
	resolvedID := uuid.New()
	svc := &mockStreamService{
		streamFn: func(_ context.Context, _ string, _ uuid.UUID, _ string) (<-chan string, uuid.UUID, error) {
			ch := make(chan string, 2)
			ch <- "hello"
			ch <- " world"
			close(ch)
			return ch, resolvedID, nil
		},
	}
	h := newTestStreamHandler(svc)
	r := newStreamRouter(h)

	rec := getStream(r, "new", "u1", "hi")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "hello") || !strings.Contains(body, " world") {
		t.Fatalf("expected chunk content in SSE body, got: %s", body)
	}
	if !strings.Contains(body, resolvedID.String()) {
		t.Fatalf("expected resolvedChatID in SSE body, got: %s", body)
	}
	if !strings.Contains(body, "completed") {
		t.Fatalf("expected done event in SSE body, got: %s", body)
	}
}

func TestStream_AccessDenied_ReturnsSSEError(t *testing.T) {
	svc := &mockStreamService{
		streamFn: func(_ context.Context, _ string, _ uuid.UUID, _ string) (<-chan string, uuid.UUID, error) {
			return nil, uuid.Nil, fmt.Errorf("service: %w", domain.ErrAccessDenied)
		},
	}
	h := newTestStreamHandler(svc)
	r := newStreamRouter(h)

	chatID := uuid.New()
	rec := getStream(r, chatID.String(), "u1", "hi")

	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 (SSE), got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "AUTH_FAILED") {
		t.Fatalf("expected ACCESS_DENIED in SSE error, got: %s", rec.Body.String())
	}
}

func TestStream_NotFound_ReturnsSSEError(t *testing.T) {
	svc := &mockStreamService{
		streamFn: func(_ context.Context, _ string, _ uuid.UUID, _ string) (<-chan string, uuid.UUID, error) {
			return nil, uuid.Nil, fmt.Errorf("service: %w", domain.ErrNotFound)
		},
	}
	h := newTestStreamHandler(svc)
	r := newStreamRouter(h)

	chatID := uuid.New()
	rec := getStream(r, chatID.String(), "u1", "hi")

	if !strings.Contains(rec.Body.String(), "CHAT_NOT_FOUND") {
		t.Fatalf("expected NOT_FOUND in SSE error, got: %s", rec.Body.String())
	}
}
