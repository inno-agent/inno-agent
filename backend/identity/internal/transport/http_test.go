package transport_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/provider"
	"github.com/inno-agent/identity/internal/refresh"
	"github.com/inno-agent/identity/internal/transport"
	"github.com/inno-agent/identity/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func makeTestIssuer(t *testing.T) *issuer.Issuer {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	der, err := x509.MarshalPKCS8PrivateKey(key)
	require.NoError(t, err)
	pemBytes := pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der})
	iss, err := issuer.New(pemBytes, 30*time.Minute)
	require.NoError(t, err)
	return iss
}

// --- stub provider ---

type stubProvider struct {
	identity provider.ExternalIdentity
	err      error
}

func (s *stubProvider) Validate(_ context.Context, _ string) (provider.ExternalIdentity, error) {
	return s.identity, s.err
}

// --- stub user service ---

type stubUserSvc struct {
	upsertUser user.User
	upsertErr  error
}

func (s *stubUserSvc) UpsertIdentity(_ context.Context, _, _, _ string) (user.User, error) {
	return s.upsertUser, s.upsertErr
}

// --- in-memory refresh store ---

type memRow struct {
	id         string
	userID     string
	expiresAt  time.Time
	revokedAt  *time.Time
	replacedBy *string
}

// memRefreshStore is an in-memory implementation of transport.RefreshStore.
type memRefreshStore struct {
	rows map[string]*memRow // key: string(hash)
	seq  int
}

var _ transport.RefreshStore = (*memRefreshStore)(nil)

func newMemRefreshStore() *memRefreshStore {
	return &memRefreshStore{rows: make(map[string]*memRow)}
}

func (m *memRefreshStore) nextID() string {
	m.seq++
	return "refresh-id-" + strconv.Itoa(m.seq)
}

func (m *memRefreshStore) Store(_ context.Context, userID string, hash []byte, expiresAt time.Time) error {
	m.rows[string(hash)] = &memRow{
		id:        m.nextID(),
		userID:    userID,
		expiresAt: expiresAt,
	}
	return nil
}

func (m *memRefreshStore) Lookup(_ context.Context, hash []byte) (refresh.Row, error) {
	row, ok := m.rows[string(hash)]
	if !ok {
		return refresh.Row{}, refresh.ErrNotFound
	}

	result := refresh.Row{
		ID:         row.id,
		UserID:     row.userID,
		ExpiresAt:  row.expiresAt,
		RevokedAt:  row.revokedAt,
		ReplacedBy: row.replacedBy,
	}

	if row.revokedAt != nil {
		return result, refresh.ErrRevoked
	}

	if time.Now().After(row.expiresAt) {
		return result, refresh.ErrExpired
	}

	return result, nil
}

func (m *memRefreshStore) Rotate(_ context.Context, oldHash, newHash []byte, newExpiresAt time.Time, userID string) (string, error) {
	newID := m.nextID()
	m.rows[string(newHash)] = &memRow{
		id:        newID,
		userID:    userID,
		expiresAt: newExpiresAt,
	}

	if row, ok := m.rows[string(oldHash)]; ok {
		now := time.Now()
		row.revokedAt = &now
		row.replacedBy = &newID
	}

	return newID, nil
}

func (m *memRefreshStore) Revoke(_ context.Context, hash []byte) error {
	row, ok := m.rows[string(hash)]
	if !ok {
		return nil
	}

	now := time.Now()
	row.revokedAt = &now

	return nil
}

func (m *memRefreshStore) RevokeChainFromID(_ context.Context, startID string) error {
	now := time.Now()

	current := startID
	for current != "" {
		var next *string

		for _, row := range m.rows {
			if row.id == current && row.revokedAt == nil {
				row.revokedAt = &now
				next = row.replacedBy

				break
			}
		}

		if next == nil {
			break
		}

		current = *next
	}

	return nil
}

// buildRouter wires up a gin engine with the given stubs.
func buildRouter(
	t *testing.T,
	p provider.AuthProvider,
	svc transport.ExchangeServicer,
	iss *issuer.Issuer,
	store transport.RefreshStore,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	transport.RegisterHTTPRoutes(r, p, svc, iss, 30*time.Minute, testOIDCEndpoints(), store, 720*time.Hour)
	return r
}

func testOIDCEndpoints() transport.OIDCEndpoints {
	return transport.OIDCEndpoints{
		Authority: "https://auth.localhost/application/o/inno-agent/",
		ClientID:  "my-client-id",
	}
}

// --- tests ---

func TestHTTP_Exchange_Success(t *testing.T) {
	iss := makeTestIssuer(t)
	p := &stubProvider{identity: provider.ExternalIdentity{
		Provider: "authentik", Sub: "user-123", Email: "user@example.com",
	}}
	svc := &stubUserSvc{upsertUser: user.User{ID: "uuid-abc"}}
	store := newMemRefreshStore()
	r := buildRouter(t, p, svc, iss, store)

	body := `{"token": "any-idp-token"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/exchange", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["access_token"])
	assert.Equal(t, float64(1800), resp["expires_in"])
	assert.NotEmpty(t, resp["refresh_token"])
	assert.NotEmpty(t, resp["refresh_expires_in"])
}

func TestHTTP_Exchange_InvalidToken(t *testing.T) {
	iss := makeTestIssuer(t)
	p := &stubProvider{err: errors.New("token expired")}
	store := newMemRefreshStore()
	r := buildRouter(t, p, nil, iss, store)

	body := `{"token": "bad-token"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/exchange", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "invalid_token", resp["error"])
}

func TestHTTP_Config(t *testing.T) {
	iss := makeTestIssuer(t)
	store := newMemRefreshStore()
	r := buildRouter(t, &stubProvider{}, nil, iss, store)

	req := httptest.NewRequest(http.MethodGet, "/identity/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "https://auth.localhost/application/o/inno-agent/", resp["authority"])
	assert.Equal(t, "my-client-id", resp["client_id"])
}

func TestHTTP_Refresh_ValidToken_Rotates(t *testing.T) {
	iss := makeTestIssuer(t)
	store := newMemRefreshStore()

	// Seed a refresh token directly.
	pt, hash, err := refresh.Mint()
	require.NoError(t, err)
	require.NoError(t, store.Store(context.Background(), "user-xyz", hash, time.Now().Add(time.Hour)))

	r := buildRouter(t, &stubProvider{}, nil, iss, store)

	body := `{"refresh_token":"` + pt + `"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["access_token"])
	newRefresh, _ := resp["refresh_token"].(string)
	assert.NotEmpty(t, newRefresh)
	assert.NotEqual(t, pt, newRefresh, "refresh token must rotate")

	// Old token must be rejected.
	req2 := httptest.NewRequest(http.MethodPost, "/identity/v1/refresh", strings.NewReader(body))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}

func TestHTTP_Refresh_ExpiredToken_401(t *testing.T) {
	iss := makeTestIssuer(t)
	store := newMemRefreshStore()

	pt, hash, err := refresh.Mint()
	require.NoError(t, err)
	require.NoError(t, store.Store(context.Background(), "user-exp", hash, time.Now().Add(-time.Second)))

	r := buildRouter(t, &stubProvider{}, nil, iss, store)

	body := `{"refresh_token":"` + pt + `"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHTTP_Refresh_UnknownToken_401(t *testing.T) {
	iss := makeTestIssuer(t)
	store := newMemRefreshStore()
	r := buildRouter(t, &stubProvider{}, nil, iss, store)

	body := `{"refresh_token":"completely-unknown-token"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/refresh", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHTTP_Revoke_Works(t *testing.T) {
	iss := makeTestIssuer(t)
	store := newMemRefreshStore()

	pt, hash, err := refresh.Mint()
	require.NoError(t, err)
	require.NoError(t, store.Store(context.Background(), "user-rev", hash, time.Now().Add(time.Hour)))

	r := buildRouter(t, &stubProvider{}, nil, iss, store)

	// Revoke.
	revokeBody := `{"refresh_token":"` + pt + `"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/revoke", strings.NewReader(revokeBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	assert.Equal(t, http.StatusNoContent, w.Code)

	// Revoked token must be rejected on /refresh.
	req2 := httptest.NewRequest(http.MethodPost, "/identity/v1/refresh", strings.NewReader(revokeBody))
	req2.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	r.ServeHTTP(w2, req2)
	assert.Equal(t, http.StatusUnauthorized, w2.Code)
}
