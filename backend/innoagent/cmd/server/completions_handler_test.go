package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"innoagent/internal/orchestrator"

	"go.uber.org/zap"
)

type stubCompletionsService struct {
	result *orchestrator.CompleteResult
	err    error
}

func (s *stubCompletionsService) Complete(ctx context.Context, body []byte) (*orchestrator.CompleteResult, error) {
	return s.result, s.err
}

func TestCompletionsHandlerMapsSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"invalid body", orchestrator.ErrInvalidBody, http.StatusBadRequest},
		{"empty messages", orchestrator.ErrEmptyMessages, http.StatusBadRequest},
		{"streaming", orchestrator.ErrStreamUnsupported, http.StatusNotImplemented},
		{"model not allowed", orchestrator.ErrModelNotAllowed, http.StatusForbidden},
		{"unexpected", errors.New("boom"), http.StatusInternalServerError},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			h := completionsHandler(&stubCompletionsService{err: tt.err}, 1<<20, zap.NewNop())

			rec := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
			h.ServeHTTP(rec, req)

			if rec.Code != tt.want {
				t.Errorf("status = %d, want %d", rec.Code, tt.want)
			}
		})
	}
}

// An unexpected error must not leak its text to the client — it may carry
// internal detail. The sentinel cases may describe themselves.
func TestCompletionsHandlerHidesUnexpectedErrorText(t *testing.T) {
	h := completionsHandler(&stubCompletionsService{
		err: errors.New("dial tcp 10.0.0.5:5432: connection refused"),
	}, 1<<20, zap.NewNop())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	h.ServeHTTP(rec, req)

	if strings.Contains(rec.Body.String(), "10.0.0.5") {
		t.Errorf("internal detail leaked to client: %s", rec.Body.String())
	}
}

func TestCompletionsHandlerWritesResultVerbatim(t *testing.T) {
	body := []byte(`{"choices":[{"message":{"tool_calls":[{"id":"c1"}]}}]}`)
	h := completionsHandler(&stubCompletionsService{
		result: &orchestrator.CompleteResult{Status: 200, Body: body, ContentType: "application/json"},
	}, 1<<20, zap.NewNop())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	h.ServeHTTP(rec, req)

	if rec.Code != 200 {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if rec.Body.String() != string(body) {
		t.Errorf("body = %q, want %q", rec.Body.String(), body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q", ct)
	}
}

// The upstream content type must be echoed, not hardcoded: labelling an
// unlabelled or non-JSON body as JSON relays a lie to the client.
func TestCompletionsHandlerEchoesUpstreamContentType(t *testing.T) {
	h := completionsHandler(&stubCompletionsService{
		result: &orchestrator.CompleteResult{Status: 200, Body: []byte("hi"), ContentType: "text/plain; charset=utf-8"},
	}, 1<<20, zap.NewNop())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader([]byte(`{}`)))
	h.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); ct != "text/plain; charset=utf-8" {
		t.Errorf("content-type = %q, want it echoed from upstream", ct)
	}
}

func TestCompletionsHandlerRejectsOversizedBody(t *testing.T) {
	h := completionsHandler(&stubCompletionsService{}, 10, zap.NewNop())

	big, err := json.Marshal(map[string]string{"pad": strings.Repeat("a", 64)})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/chat/completions", bytes.NewReader(big))
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Errorf("status = %d, want 413", rec.Code)
	}
}

func TestCompletionsHandlerRejectsNonPost(t *testing.T) {
	h := completionsHandler(&stubCompletionsService{}, 1<<20, zap.NewNop())

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/v1/chat/completions", nil)
	h.ServeHTTP(rec, req)

	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("status = %d, want 405", rec.Code)
	}
}
