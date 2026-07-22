package mastra

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
)

func TestClient_Review_SendsBearerAndTrimsURL(t *testing.T) {
	var gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewEncoder(w).Encode(map[string]string{"review_markdown": "ok"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL+"/", "secret-xyz")
	out, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "")
	if err != nil {
		t.Fatalf("Review: %v", err)
	}
	if out != "ok" {
		t.Fatalf("markdown = %q", out)
	}
	if gotAuth != "Bearer secret-xyz" {
		t.Fatalf("Authorization = %q, want %q", gotAuth, "Bearer secret-xyz")
	}
}

func TestReviewSendsDelegatedTokenSeparateFromAuth(t *testing.T) {
	var gotAuth, gotDelegated string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotDelegated = r.Header.Get("X-Delegated-Token")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"review_markdown":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "shared-secret")
	if _, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "user-token"); err != nil {
		t.Fatalf("Review: %v", err)
	}

	if gotAuth != "Bearer shared-secret" {
		t.Errorf("Authorization = %q, want the shared secret", gotAuth)
	}
	if gotDelegated != "user-token" {
		t.Errorf("X-Delegated-Token = %q, want user-token", gotDelegated)
	}
}

func TestReviewOmitsDelegatedHeaderWhenTokenEmpty(t *testing.T) {
	var present bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, present = r.Header["X-Delegated-Token"]
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"review_markdown":"ok"}`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "shared-secret")
	if _, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, ""); err != nil {
		t.Fatalf("Review: %v", err)
	}
	if present {
		t.Error("X-Delegated-Token sent despite empty token")
	}
}

func TestReviewClassifies401AsTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`unauthorized`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "shared-secret")
	_, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "expired")
	if !errors.Is(err, domain.ErrTransient) {
		t.Fatalf("err = %v, want ErrTransient (an expired delegated token is retryable)", err)
	}
}

func TestReviewClassifies403AsPermanent(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`forbidden`))
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "shared-secret")
	_, err := c.Review(context.Background(), domain.PRRef{Owner: "o", Repo: "r", Index: 1}, "valid")
	if !errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("err = %v, want ErrPermanent (token valid, model/quota refused)", err)
	}
}
