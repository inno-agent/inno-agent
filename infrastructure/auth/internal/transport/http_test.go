package transport_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inno-agent/auth/internal/provider"
	"github.com/inno-agent/auth/internal/transport"
	"github.com/inno-agent/auth/internal/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

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
	ctx        user.UserContext
	ctxErr     error
}

func (s *stubUserSvc) UpsertIdentity(_ context.Context, _, _, _ string) (user.User, error) {
	return s.upsertUser, s.upsertErr
}
func (s *stubUserSvc) GetContext(_ context.Context, _ string) (user.UserContext, error) {
	return s.ctx, s.ctxErr
}
func (s *stubUserSvc) UpdateContext(_ context.Context, _ string, _ []byte) error { return nil }

// --- test ---

func TestHTTP_Exchange_Success(t *testing.T) {
	gin.SetMode(gin.TestMode)
	iss := makeTestIssuer(t)

	p := &stubProvider{identity: provider.ExternalIdentity{
		Provider: "zitadel", Sub: "user-123", Email: "user@example.com",
	}}
	svc := &stubUserSvc{
		upsertUser: user.User{ID: "uuid-abc", Tier: "user"},
		ctx:        user.UserContext{UserID: "uuid-abc", Version: 1},
	}

	r := gin.New()
	transport.RegisterHTTPRoutes(r, p, svc, iss, 30*time.Minute, "http://localhost:8080", "test-client")

	body := `{"token": "any-idp-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/v1/exchange", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["access_token"])
	assert.Equal(t, float64(1800), resp["expires_in"])
}

func TestHTTP_Exchange_InvalidToken(t *testing.T) {
	gin.SetMode(gin.TestMode)
	iss := makeTestIssuer(t)

	p := &stubProvider{err: errors.New("token expired")}
	r := gin.New()
	transport.RegisterHTTPRoutes(r, p, nil, iss, 30*time.Minute, "", "")

	body := `{"token": "bad-token"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/v1/exchange", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "invalid_token", resp["error"])
}

func TestHTTP_Config(t *testing.T) {
	gin.SetMode(gin.TestMode)
	iss := makeTestIssuer(t)

	r := gin.New()
	transport.RegisterHTTPRoutes(r, &stubProvider{}, nil, iss, 30*time.Minute, "http://localhost:8080", "my-client-id")

	req := httptest.NewRequest(http.MethodGet, "/auth/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "http://localhost:8080", resp["authority"])
	assert.Equal(t, "my-client-id", resp["client_id"])
}
