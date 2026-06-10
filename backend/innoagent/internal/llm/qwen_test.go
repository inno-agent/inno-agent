package llm_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"innoagent/internal/llm"
)

func makeServer(
	t *testing.T,
	status int,
	body any,
) *httptest.Server {
	t.Helper()

	srv := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			w.Header().Set(
				"Content-Type",
				"application/json",
			)

			w.WriteHeader(status)

			if err := json.NewEncoder(w).Encode(body); err != nil {
				t.Errorf(
					"test server encode error: %v",
					err,
				)
			}
		}),
	)

	t.Cleanup(srv.Close)

	return srv
}

func TestChat_Success(t *testing.T) {
	want := "Hello from Qwen!"

	resp := llm.ChatResponse{
		Choices: []llm.Choice{
			{
				Message: llm.ChatMessage{
					Role:    "assistant",
					Content: want,
				},
			},
		},
	}

	srv := makeServer(
		t,
		http.StatusOK,
		resp,
	)

	provider := llm.NewQwenProvider(
		srv.URL+"/v1",
		llm.WithModel("test-model"),
	)

	got, err := provider.Chat(
		context.Background(),
		[]llm.Message{{Role: "user", Content: "Hello"}},
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if got != want {
		t.Errorf(
			"got %q, want %q",
			got,
			want,
		)
	}
}

func TestChat_EmptyMessage(t *testing.T) {
	provider := llm.NewQwenProvider(
		"http://localhost:8000/v1",
		llm.WithModel("test-model"),
	)

	_, err := provider.Chat(
		context.Background(),
		[]llm.Message{},
	)

	if !errors.Is(err, llm.ErrEmptyMessage) {
		t.Errorf(
			"expected ErrEmptyMessage, got %v",
			err,
		)
	}
}

func TestChat_EmptyChoices(t *testing.T) {
	resp := llm.ChatResponse{
		Choices: []llm.Choice{},
	}

	srv := makeServer(
		t,
		http.StatusOK,
		resp,
	)

	provider := llm.NewQwenProvider(
		srv.URL+"/v1",
		llm.WithModel("test-model"),
	)

	_, err := provider.Chat(
		context.Background(),
		[]llm.Message{{Role: "user", Content: "Hi"}},
	)

	if !errors.Is(err, llm.ErrEmptyResponse) {
		t.Errorf(
			"expected ErrEmptyResponse, got %v",
			err,
		)
	}
}

func TestChat_UpstreamError(t *testing.T) {
	errBody := map[string]any{
		"error": map[string]any{
			"message": "model not found",
			"type":    "invalid_request_error",
		},
	}

	srv := makeServer(
		t,
		http.StatusNotFound,
		errBody,
	)

	provider := llm.NewQwenProvider(
		srv.URL+"/v1",
		llm.WithModel("test-model"),
	)

	_, err := provider.Chat(
		context.Background(),
		[]llm.Message{{Role: "user", Content: "Hi"}},
	)

	var provErr *llm.ProviderError

	if !errors.As(err, &provErr) {
		t.Fatalf(
			"expected *ProviderError, got %T: %v",
			err,
			err,
		)
	}

	if provErr.StatusCode != http.StatusNotFound {
		t.Errorf(
			"expected 404, got %d",
			provErr.StatusCode,
		)
	}

	if provErr.Message != "model not found" {
		t.Errorf(
			"unexpected message: %q",
			provErr.Message,
		)
	}
}

func TestChat_ContextCancelled(t *testing.T) {
	srv := httptest.NewServer(
		http.HandlerFunc(func(
			w http.ResponseWriter,
			r *http.Request,
		) {
			<-r.Context().Done()
		}),
	)

	t.Cleanup(srv.Close)

	ctx, cancel := context.WithCancel(
		context.Background(),
	)

	cancel()

	provider := llm.NewQwenProvider(
		srv.URL+"/v1",
		llm.WithModel("test-model"),
	)

	_, err := provider.Chat(
		ctx,
		[]llm.Message{{Role: "user", Content: "Hello"}},
	)

	if err == nil {
		t.Fatal(
			"expected error for cancelled context, got nil",
		)
	}
}

func TestChat_CustomModel(t *testing.T) {
	want := "Custom model response"

	resp := llm.ChatResponse{
		Choices: []llm.Choice{
			{
				Message: llm.ChatMessage{
					Role:    "assistant",
					Content: want,
				},
			},
		},
	}

	srv := makeServer(
		t,
		http.StatusOK,
		resp,
	)

	provider := llm.NewQwenProvider(
		srv.URL+"/v1",
		llm.WithModel("my-custom-model"),
	)

	got, err := provider.Chat(
		context.Background(),
		[]llm.Message{{Role: "user", Content: "test message"}},
	)
	if err != nil {
		t.Fatalf(
			"unexpected error: %v",
			err,
		)
	}

	if got != want {
		t.Errorf(
			"got %q, want %q",
			got,
			want,
		)
	}
}

func TestProviderInterface(t *testing.T) {
	var _ llm.Provider = llm.NewQwenProvider(
		"http://localhost:8000/v1",
		llm.WithModel("test-model"),
	)
}
