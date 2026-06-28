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

func makeIdentitySrv(t *testing.T, token string, status int) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": token,
				"expires_in":   3600,
			})
		}
	}))
}

func ref(assigner string) domain.PRRef {
	return domain.PRRef{Owner: "org", Repo: "repo", Index: 1, Assigner: assigner}
}

func TestService_Token_Success(t *testing.T) {
	store := &fakeUserStore{m: map[string]string{"alice": "user-uuid-1"}}
	srv := makeIdentitySrv(t, "service-jwt-token", http.StatusOK)
	defer srv.Close()

	ts := tokensource.NewService(store, srv.URL, "review-consumer", "secret")
	tok, userID, err := ts.Token(context.Background(), ref("alice"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "service-jwt-token" {
		t.Fatalf("expected service-jwt-token, got %q", tok)
	}
	if userID != "user-uuid-1" {
		t.Fatalf("expected user-uuid-1, got %q", userID)
	}
}

func TestService_Token_NotOnboarded(t *testing.T) {
	store := &fakeUserStore{m: map[string]string{}}
	srv := makeIdentitySrv(t, "", http.StatusOK)
	defer srv.Close()

	ts := tokensource.NewService(store, srv.URL, "review-consumer", "secret")
	_, _, err := ts.Token(context.Background(), ref("unknown"))

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
	_, _, err := ts.Token(context.Background(), ref("bob"))

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
	_, _, err := ts.Token(context.Background(), ref("carol"))

	if !errors.Is(err, domain.ErrPermanent) {
		t.Fatalf("expected ErrPermanent for 401, got %v", err)
	}
}

func TestService_Token_CachesServiceJWT(t *testing.T) {
	store := &fakeUserStore{m: map[string]string{"dave": "user-uuid-4"}}
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token": fmt.Sprintf("tok-%d", calls),
			"expires_in":   3600,
		})
	}))
	defer srv.Close()

	ts := tokensource.NewService(store, srv.URL, "review-consumer", "secret")
	tok1, _, _ := ts.Token(context.Background(), ref("dave"))
	tok2, _, _ := ts.Token(context.Background(), ref("dave"))

	if calls != 1 {
		t.Fatalf("expected 1 identity call (cache hit), got %d", calls)
	}
	if tok1 != tok2 {
		t.Fatalf("expected same token from cache, got %q and %q", tok1, tok2)
	}
}
