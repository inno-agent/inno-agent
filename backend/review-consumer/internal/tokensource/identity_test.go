package tokensource_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/tokensource"
)

func ref(assigner string) domain.PRRef {
	return domain.PRRef{Assigner: assigner}
}

func makeOKHandler(t *testing.T, secret, token string, expiresIn int) http.HandlerFunc {
	t.Helper()

	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Service-Secret") != secret {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": token,
			"expires_in":   expiresIn,
		})
	}
}

func TestIdentity_Token_200_ReturnsToken(t *testing.T) {
	srv := httptest.NewServer(makeOKHandler(t, "s3cr3t", "tok-abc", 300))
	defer srv.Close()

	ts := tokensource.NewIdentity(srv.URL, "s3cr3t", srv.Client())
	tok, err := ts.Token(context.Background(), ref("alice"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if tok != "tok-abc" {
		t.Fatalf("expected tok-abc, got %q", tok)
	}
}

func TestIdentity_Token_200_CachesToken(t *testing.T) {
	var calls atomic.Int32

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok-cached",
			"expires_in":   600,
		})
	}))
	defer srv.Close()

	ts := tokensource.NewIdentity(srv.URL, "", srv.Client())

	// Two calls for the same assigner should only hit the server once.
	_, err := ts.Token(context.Background(), ref("bob"))
	if err != nil {
		t.Fatalf("first call: %v", err)
	}

	_, err = ts.Token(context.Background(), ref("bob"))
	if err != nil {
		t.Fatalf("second call: %v", err)
	}

	if calls.Load() != 1 {
		t.Fatalf("expected 1 HTTP call (cached), got %d", calls.Load())
	}
}

func TestIdentity_Token_404_ErrNotOnboarded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	ts := tokensource.NewIdentity(srv.URL, "", srv.Client())
	_, err := ts.Token(context.Background(), ref("carol"))

	if !errors.Is(err, domain.ErrNotOnboarded) {
		t.Fatalf("expected ErrNotOnboarded, got %v", err)
	}
}

func TestIdentity_Token_5xx_ErrTransient(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ts := tokensource.NewIdentity(srv.URL, "", srv.Client())
	_, err := ts.Token(context.Background(), ref("dave"))

	if !errors.Is(err, domain.ErrTransient) {
		t.Fatalf("expected ErrTransient, got %v", err)
	}
}

func TestIdentity_Token_Expiry_Refetches(t *testing.T) {
	var calls atomic.Int32
	// expiresIn=1 → effectively expires immediately (< 30 s threshold makes it
	// negative, so the cached entry is always stale).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"access_token": "tok-fresh",
			"expires_in":   1, // will be treated as stale immediately
		})
	}))
	defer srv.Close()

	ts := tokensource.NewIdentity(srv.URL, "", srv.Client())

	_, _ = ts.Token(context.Background(), ref("eve"))

	// Because expires_in=1 and we subtract 30s, the cached exp is in the past.
	// A brief sleep ensures time.Now() is past the cached expiry.
	time.Sleep(5 * time.Millisecond)

	_, _ = ts.Token(context.Background(), ref("eve"))

	if calls.Load() < 2 {
		t.Fatalf("expected at least 2 HTTP calls (expired cache forces refetch), got %d", calls.Load())
	}
}
