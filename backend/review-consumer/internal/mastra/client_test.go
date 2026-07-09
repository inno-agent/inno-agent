package mastra_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/mastra"
)

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c := mastra.NewClient("http://localhost:4100/")
	// Client should be usable — no panic
	_ = c
}

func TestReview_Success(t *testing.T) {
	var gotBody map[string]interface{}
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{
			"review_markdown": "# Review\nLooks good!",
		})
	}))
	defer srv.Close()

	c := mastra.NewClient(srv.URL)
	result, err := c.Review(context.Background(), domain.PRRef{
		Owner:   "myorg",
		Repo:    "myrepo",
		Index:   42,
		HeadSHA: "abc123",
	}, "delegated-jwt")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result != "# Review\nLooks good!" {
		t.Fatalf("unexpected result: %q", result)
	}
	if gotAuth != "Bearer delegated-jwt" {
		t.Fatalf("expected Bearer token, got %q", gotAuth)
	}
	if gotBody["owner"] != "myorg" {
		t.Fatalf("expected owner=myorg, got %v", gotBody["owner"])
	}
	if gotBody["repo"] != "myrepo" {
		t.Fatalf("expected repo=myrepo, got %v", gotBody["repo"])
	}
	if gotBody["pullNumber"] != float64(42) {
		t.Fatalf("expected pullNumber=42, got %v", gotBody["pullNumber"])
	}
	if gotBody["headSha"] != "abc123" {
		t.Fatalf("expected headSha=abc123, got %v", gotBody["headSha"])
	}
}

func TestReview_EmptyToken_NoAuthHeader(t *testing.T) {
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"review_markdown": "ok"})
	}))
	defer srv.Close()

	c := mastra.NewClient(srv.URL)
	_, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "")

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotAuth != "" {
		t.Fatalf("expected no auth header, got %q", gotAuth)
	}
}

func TestReview_ServerError_5xx_Transient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := mastra.NewClient(srv.URL)
	_, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "tok")

	if err == nil {
		t.Fatal("expected error for 500")
	}
	if !errors.Is(err, domain.ErrTransient) {
		t.Fatalf("expected ErrTransient, got %v", err)
	}
}

func TestReview_ServerError_4xx_Permanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadRequest)
	}))
	defer srv.Close()

	c := mastra.NewClient(srv.URL)
	_, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "tok")

	if err == nil {
		t.Fatal("expected error for 400")
	}
	if !errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("expected ErrPermanent, got %v", err)
	}
}

func TestReview_Unauthorized_4xx_Permanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := mastra.NewClient(srv.URL)
	_, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "bad-token")

	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("expected ErrPermanent, got %v", err)
	}
}

func TestReview_InvalidJSON_Response(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte("not json"))
	}))
	defer srv.Close()

	c := mastra.NewClient(srv.URL)
	_, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "tok")

	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
	if !strings.Contains(err.Error(), "decode") {
		t.Fatalf("expected decode error, got %v", err)
	}
}

func TestReview_ConnectionRefused_Transient(t *testing.T) {
	c := mastra.NewClient("http://localhost:1") // nothing listening
	_, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "tok")

	if err == nil {
		t.Fatal("expected error for connection refused")
	}
	if !errors.Is(err, domain.ErrTransient) {
		t.Fatalf("expected ErrTransient, got %v", err)
	}
}

func TestReview_ContextCanceled(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Simulate slow response
		select {}
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	c := mastra.NewClient(srv.URL)
	_, err := c.Review(ctx, domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "tok")

	if err == nil {
		t.Fatal("expected error for canceled context")
	}
}
