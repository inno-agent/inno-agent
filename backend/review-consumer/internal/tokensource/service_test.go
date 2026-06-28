package tokensource_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/tokensource"
)

type fakeUserStore struct {
	m map[string]string // gitflame_username -> user_id
}

func (f *fakeUserStore) GetUserID(_ context.Context, gitflameUsername string) (string, bool, error) {
	uid, ok := f.m[gitflameUsername]
	return uid, ok, nil
}

// makeIdentityServer creates a test identity server that handles both
// /identity/v1/service-token and /identity/v1/token endpoints.
func makeIdentityServer(t *testing.T, svcStatus, exchangeStatus int) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	mux.HandleFunc("/identity/v1/service-token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(svcStatus)
		if svcStatus == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "svc-jwt",
				"expires_in":   3600,
			})
		}
	})
	mux.HandleFunc("/identity/v1/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(exchangeStatus)
		if exchangeStatus == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "delegated-jwt",
				"expires_in":   900,
			})
		}
	})
	return httptest.NewServer(mux)
}

func ref(assigner string) domain.PRRef {
	return domain.PRRef{Owner: "org", Repo: "repo", Index: 1, Assigner: assigner}
}

func TestService_Token_Success(t *testing.T) {
	store := &fakeUserStore{m: map[string]string{"alice": "user-uuid-1"}}
	srv := makeIdentityServer(t, http.StatusOK, http.StatusOK)
	defer srv.Close()

	ts := tokensource.NewService(store, srv.URL, "review-consumer", "secret")
	tok, err := ts.Token(context.Background(), ref("alice"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "delegated-jwt" {
		t.Fatalf("expected delegated-jwt, got %q", tok)
	}
}

func TestService_Token_NotOnboarded(t *testing.T) {
	store := &fakeUserStore{m: map[string]string{}}
	srv := makeIdentityServer(t, http.StatusOK, http.StatusOK)
	defer srv.Close()

	ts := tokensource.NewService(store, srv.URL, "review-consumer", "secret")
	_, err := ts.Token(context.Background(), ref("unknown"))

	if !errors.Is(err, domain.ErrNotOnboarded) {
		t.Fatalf("expected ErrNotOnboarded, got %v", err)
	}
}

func TestService_Token_IdentityDown_Transient(t *testing.T) {
	store := &fakeUserStore{m: map[string]string{"bob": "user-uuid-2"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	ts := tokensource.NewService(store, srv.URL, "review-consumer", "secret")
	_, err := ts.Token(context.Background(), ref("bob"))

	if !errors.Is(err, domain.ErrTransient) {
		t.Fatalf("expected ErrTransient, got %v", err)
	}
}

func TestService_Token_InvalidCredentials_Permanent(t *testing.T) {
	store := &fakeUserStore{m: map[string]string{"carol": "user-uuid-3"}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	ts := tokensource.NewService(store, srv.URL, "review-consumer", "wrong-secret")
	_, err := ts.Token(context.Background(), ref("carol"))

	if !errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("expected ErrPermanent for 401, got %v", err)
	}
}

func TestService_Token_CachesServiceJWT(t *testing.T) {
	store := &fakeUserStore{m: map[string]string{
		"dave": "user-uuid-4",
		"eve":  "user-uuid-5",
	}}
	svcCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/identity/v1/service-token", func(w http.ResponseWriter, r *http.Request) {
		svcCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("svc-tok-%d", svcCalls),
			"expires_in":   3600,
		})
	})
	mux.HandleFunc("/identity/v1/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "delegated", "expires_in": 900})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ts := tokensource.NewService(store, srv.URL, "review-consumer", "secret")
	_, _ = ts.Token(context.Background(), ref("dave"))
	_, _ = ts.Token(context.Background(), ref("eve"))

	if svcCalls != 1 {
		t.Fatalf("service-token should be called once (cached), got %d", svcCalls)
	}
}

func TestService_Token_CachesDelegatePerUser(t *testing.T) {
	store := &fakeUserStore{m: map[string]string{"frank": "user-uuid-6"}}
	exchangeCalls := 0
	mux := http.NewServeMux()
	mux.HandleFunc("/identity/v1/service-token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": "svc-tok", "expires_in": 3600})
	})
	mux.HandleFunc("/identity/v1/token", func(w http.ResponseWriter, r *http.Request) {
		exchangeCalls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("delegated-%d", exchangeCalls),
			"expires_in":   900,
		})
	})
	srv := httptest.NewServer(mux)
	defer srv.Close()

	ts := tokensource.NewService(store, srv.URL, "review-consumer", "secret")
	tok1, _ := ts.Token(context.Background(), ref("frank"))
	tok2, _ := ts.Token(context.Background(), ref("frank"))

	if exchangeCalls != 1 {
		t.Fatalf("exchange should be called once (cached per user), got %d", exchangeCalls)
	}
	if tok1 != tok2 {
		t.Fatalf("expected same cached token, got %q and %q", tok1, tok2)
	}
}
