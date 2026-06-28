package tokensource_test

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/identityclient"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/secretbox"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/tokensource"
)

func ref(assigner string) domain.PRRef {
	return domain.PRRef{Assigner: assigner}
}

func makeCrypter(t *testing.T) *secretbox.SecretBox {
	t.Helper()
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		t.Fatalf("rand: %v", err)
	}
	sb, err := secretbox.NewFromBase64Key(base64.StdEncoding.EncodeToString(key))
	if err != nil {
		t.Fatalf("secretbox: %v", err)
	}
	return sb
}

// fakeStore is an in-memory installation store.
type fakeStore struct {
	rows    map[string]tokensource.InstallationRow
	updated map[string]tokensource.InstallationRow
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		rows:    make(map[string]tokensource.InstallationRow),
		updated: make(map[string]tokensource.InstallationRow),
	}
}

func (f *fakeStore) Get(_ context.Context, username string) (tokensource.InstallationRow, bool, error) {
	row, ok := f.rows[username]
	return row, ok, nil
}

func (f *fakeStore) UpdateRefresh(_ context.Context, username string, ciphertext, nonce []byte) error {
	f.updated[username] = tokensource.InstallationRow{
		GitFlameUsername:  username,
		RefreshCiphertext: ciphertext,
		RefreshNonce:      nonce,
	}
	// Also reflect into rows so subsequent Get sees the rotation.
	f.rows[username] = f.updated[username]
	return nil
}

// seed encrypts plainRefresh and registers it for username.
func (f *fakeStore) seed(t *testing.T, sb *secretbox.SecretBox, username, plainRefresh string) {
	t.Helper()
	ct, nonce, err := sb.Encrypt([]byte(plainRefresh))
	if err != nil {
		t.Fatalf("seed encrypt: %v", err)
	}
	f.rows[username] = tokensource.InstallationRow{
		GitFlameUsername:  username,
		RefreshCiphertext: ct,
		RefreshNonce:      nonce,
	}
}

func TestInstallation_Onboarded_RefreshesAndRotates(t *testing.T) {
	sb := makeCrypter(t)
	store := newFakeStore()
	store.seed(t, sb, "alice", "old-refresh-token")

	// identity /refresh returns a fresh access + rotated refresh.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			RefreshToken string `json:"refresh_token"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.RefreshToken != "old-refresh-token" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":       "fresh-access",
			"expires_in":         300,
			"refresh_token":      "rotated-refresh-token",
			"refresh_expires_in": 3600,
		})
	}))
	defer srv.Close()

	idc := identityclient.New(srv.URL, srv.Client())
	ts := tokensource.NewInstallation(store, idc, sb)

	tok, err := ts.Token(context.Background(), ref("alice"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tok != "fresh-access" {
		t.Fatalf("expected fresh-access, got %q", tok)
	}

	// The rotated refresh must have been persisted (encrypted).
	up, ok := store.updated["alice"]
	if !ok {
		t.Fatal("expected rotated refresh to be persisted")
	}
	plain, err := sb.Decrypt(up.RefreshCiphertext, up.RefreshNonce)
	if err != nil {
		t.Fatalf("decrypt persisted: %v", err)
	}
	if string(plain) != "rotated-refresh-token" {
		t.Fatalf("expected rotated-refresh-token persisted, got %q", plain)
	}
}

func TestInstallation_Onboarded_CachesAccess(t *testing.T) {
	sb := makeCrypter(t)
	store := newFakeStore()
	store.seed(t, sb, "bob", "bob-refresh")

	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":       "cached-access",
			"expires_in":         600,
			"refresh_token":      "bob-rotated",
			"refresh_expires_in": 3600,
		})
	}))
	defer srv.Close()

	idc := identityclient.New(srv.URL, srv.Client())
	ts := tokensource.NewInstallation(store, idc, sb)

	_, _ = ts.Token(context.Background(), ref("bob"))
	_, _ = ts.Token(context.Background(), ref("bob"))

	if calls != 1 {
		t.Fatalf("expected 1 refresh call (cached), got %d", calls)
	}
}

func TestInstallation_NotFound_ErrNotOnboarded(t *testing.T) {
	sb := makeCrypter(t)
	store := newFakeStore() // empty

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("identity should not be called when there is no installation")
	}))
	defer srv.Close()

	idc := identityclient.New(srv.URL, srv.Client())
	ts := tokensource.NewInstallation(store, idc, sb)

	_, err := ts.Token(context.Background(), ref("carol"))
	if !errors.Is(err, domain.ErrNotOnboarded) {
		t.Fatalf("expected ErrNotOnboarded, got %v", err)
	}
}

func TestInstallation_GrantDead_ErrNotOnboarded(t *testing.T) {
	sb := makeCrypter(t)
	store := newFakeStore()
	store.seed(t, sb, "dave", "dave-refresh")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	idc := identityclient.New(srv.URL, srv.Client())
	ts := tokensource.NewInstallation(store, idc, sb)

	_, err := ts.Token(context.Background(), ref("dave"))
	if !errors.Is(err, domain.ErrNotOnboarded) {
		t.Fatalf("expected ErrNotOnboarded for 401, got %v", err)
	}
}

func TestInstallation_GrantExpired_ErrGrantExpired(t *testing.T) {
	sb := makeCrypter(t)
	store := newFakeStore()
	store.seed(t, sb, "frank", "frank-refresh")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":"token_expired"}`))
	}))
	defer srv.Close()

	idc := identityclient.New(srv.URL, srv.Client())
	ts := tokensource.NewInstallation(store, idc, sb)

	_, err := ts.Token(context.Background(), ref("frank"))
	if !errors.Is(err, domain.ErrGrantExpired) {
		t.Fatalf("expected ErrGrantExpired for token_expired 401, got %v", err)
	}
}

func TestInstallation_IdentityDown_ErrTransient(t *testing.T) {
	sb := makeCrypter(t)
	store := newFakeStore()
	store.seed(t, sb, "eve", "eve-refresh")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	idc := identityclient.New(srv.URL, srv.Client())
	ts := tokensource.NewInstallation(store, idc, sb)

	_, err := ts.Token(context.Background(), ref("eve"))
	if !errors.Is(err, domain.ErrTransient) {
		t.Fatalf("expected ErrTransient, got %v", err)
	}
}
