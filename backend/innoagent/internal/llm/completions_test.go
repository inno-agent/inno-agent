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

	wantBody := []byte(`{"choices":[{"message":{"tool_calls":[{"id":"c1"}]}}]}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(wantBody)
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
	if string(res.Body) != string(wantBody) {
		t.Errorf("body = %s, want %s", res.Body, wantBody)
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
	wantBody := []byte(`{"error":{"message":"bad tools schema"}}`)

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write(wantBody)
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
	if string(res.Body) != string(wantBody) {
		t.Errorf("body = %s, want %s", res.Body, wantBody)
	}
}

func TestCompletionsClientSizeCapBoundary(t *testing.T) {
	tests := []struct {
		name    string
		size    int
		wantErr bool
	}{
		{"one under cap", 9, false},
		{"exactly at cap", 10, false},
		{"one over cap", 11, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(strings.Repeat("x", tt.size)))
			}))
			defer srv.Close()

			c := llm.NewCompletionsClient(srv.URL+"/v1", "", 5*time.Second, 10)
			res, err := c.Complete(context.Background(), []byte(`{}`))

			if tt.wantErr {
				if !errors.Is(err, llm.ErrResponseTooLarge) {
					t.Fatalf("err = %v, want ErrResponseTooLarge", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if len(res.Body) != tt.size {
				t.Errorf("body length = %d, want %d (body must not be truncated)", len(res.Body), tt.size)
			}
		})
	}
}

func TestCompletionsClientPreservesContentType(t *testing.T) {
	tests := []struct {
		name          string
		setCType      bool
		upstreamCType string
		wantCType     string
	}{
		{"upstream json passes through", true, "application/json", "application/json"},
		{"upstream text passes through", true, "text/plain; charset=utf-8", "text/plain; charset=utf-8"},
		{"upstream missing defaults to json", false, "", "application/json"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if tt.setCType {
					w.Header().Set("Content-Type", tt.upstreamCType)
				} else {
					// Explicitly set to empty to suppress Go's auto-detection
					w.Header().Set("Content-Type", "")
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{}`))
			}))
			defer srv.Close()

			c := llm.NewCompletionsClient(srv.URL+"/v1", "", 5*time.Second, 1<<20)
			res, err := c.Complete(context.Background(), []byte(`{}`))
			if err != nil {
				t.Fatalf("Complete: %v", err)
			}
			if res.ContentType != tt.wantCType {
				t.Errorf("content type = %q, want %q", res.ContentType, tt.wantCType)
			}
		})
	}
}

func TestCompletionsClientConstructorDefaults(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"test":"data"}`))
	}))
	defer srv.Close()

	// Client built with zero arguments should still work
	c := llm.NewCompletionsClient(srv.URL+"/v1", "", 0, 0)
	res, err := c.Complete(context.Background(), []byte(`{}`))
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if res.Status != http.StatusOK {
		t.Errorf("status = %d, want 200", res.Status)
	}
}
