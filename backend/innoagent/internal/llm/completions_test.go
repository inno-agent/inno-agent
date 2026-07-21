package llm_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"innoagent/internal/llm"
)

func TestCompletionsClientForwardsBodyVerbatim(t *testing.T) {
	var gotPath, gotAuth, gotContentType string
	var gotBody []byte

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"tool_calls":[{"id":"c1"}]}}]}`))
	}))
	defer srv.Close()

	c := llm.NewCompletionsClient(srv.URL+"/v1", "secret-key", 5*time.Second, 1<<20)

	body := []byte(`{"model":"m","messages":[],"tools":[{"type":"function"}]}`)
	res, err := c.Complete(context.Background(), body)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if gotPath != "/v1/chat/completions" {
		t.Errorf("path = %q, want /v1/chat/completions", gotPath)
	}
	if gotAuth != "Bearer secret-key" {
		t.Errorf("auth = %q, want Bearer secret-key", gotAuth)
	}
	if gotContentType != "application/json" {
		t.Errorf("content-type = %q", gotContentType)
	}
	if string(gotBody) != string(body) {
		t.Errorf("body forwarded as %q, want %q", gotBody, body)
	}
	if res.Status != http.StatusOK {
		t.Errorf("status = %d, want 200", res.Status)
	}
	if !strings.Contains(string(res.Body), `"tool_calls"`) {
		t.Errorf("tool_calls lost from response: %s", res.Body)
	}
}

func TestCompletionsClientOmitsAuthWhenNoAPIKey(t *testing.T) {
	var hasAuth bool

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, hasAuth = r.Header["Authorization"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := llm.NewCompletionsClient(srv.URL+"/v1", "", 5*time.Second, 1<<20)
	if _, err := c.Complete(context.Background(), []byte(`{}`)); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if hasAuth {
		t.Error("Authorization header set despite empty API key")
	}
}

func TestCompletionsClientReturnsUpstreamStatusAndBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad tools schema"}}`))
	}))
	defer srv.Close()

	c := llm.NewCompletionsClient(srv.URL+"/v1", "", 5*time.Second, 1<<20)
	res, err := c.Complete(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Status != http.StatusBadRequest {
		t.Errorf("status = %d, want 400", res.Status)
	}
	if !strings.Contains(string(res.Body), "bad tools schema") {
		t.Errorf("upstream error body lost: %s", res.Body)
	}
}

func TestCompletionsClientRejectsOversizedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(strings.Repeat("x", 100)))
	}))
	defer srv.Close()

	c := llm.NewCompletionsClient(srv.URL+"/v1", "", 5*time.Second, 10)
	_, err := c.Complete(context.Background(), []byte(`{}`))
	if !errors.Is(err, llm.ErrResponseTooLarge) {
		t.Fatalf("err = %v, want ErrResponseTooLarge", err)
	}
}
