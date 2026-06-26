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
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/inno-agent/identity/internal/botprincipal"
	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/provider"
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

// --- stub bot principal service ---

type stubBotSvc struct {
	upsertErr error
	userID    string
	found     bool
	findErr   error
}

func (s *stubBotSvc) UpsertConsent(_ context.Context, _, _ string) error {
	return s.upsertErr
}

func (s *stubBotSvc) FindUserIDByGitFlameUsername(_ context.Context, _ string) (string, bool, error) {
	return s.userID, s.found, s.findErr
}

// buildRouter is a helper that wires up a gin engine with the given parameters.
func buildRouter(
	t *testing.T,
	p provider.AuthProvider,
	svc transport.ExchangeServicer,
	iss *issuer.Issuer,
	botSvc transport.BotPrincipalServicer,
	secret string,
) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	transport.RegisterHTTPRoutes(r, p, svc, iss, 30*time.Minute, testOIDCEndpoints(), botSvc, secret)
	return r
}

// --- existing exchange / config tests ---

func TestHTTP_Exchange_Success(t *testing.T) {
	iss := makeTestIssuer(t)

	p := &stubProvider{identity: provider.ExternalIdentity{
		Provider: "authentik", Sub: "user-123", Email: "user@example.com",
	}}
	svc := &stubUserSvc{
		upsertUser: user.User{ID: "uuid-abc"},
	}

	r := buildRouter(t, p, svc, iss, &stubBotSvc{}, "")

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
}

func TestHTTP_Exchange_InvalidToken(t *testing.T) {
	iss := makeTestIssuer(t)

	p := &stubProvider{err: errors.New("token expired")}
	r := buildRouter(t, p, nil, iss, &stubBotSvc{}, "")

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

	r := buildRouter(t, &stubProvider{}, nil, iss, &stubBotSvc{}, "")

	req := httptest.NewRequest(http.MethodGet, "/identity/v1/config", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "https://auth.localhost/application/o/inno-agent/", resp["authority"])
	assert.Equal(t, "my-client-id", resp["client_id"])
}

// --- consent endpoint tests ---

func TestHTTP_Consent_NoToken_401(t *testing.T) {
	iss := makeTestIssuer(t)
	r := buildRouter(t, &stubProvider{}, nil, iss, &stubBotSvc{}, "")

	body := `{"gitflame_username":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/bot/consent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHTTP_Consent_InvalidToken_401(t *testing.T) {
	iss := makeTestIssuer(t)
	r := buildRouter(t, &stubProvider{}, nil, iss, &stubBotSvc{}, "")

	body := `{"gitflame_username":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/bot/consent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer not-a-real-token")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHTTP_Consent_Success_204(t *testing.T) {
	iss := makeTestIssuer(t)

	// Issue a valid aicore token to use as the caller's bearer.
	tok, err := iss.Issue("user-uuid-001")
	require.NoError(t, err)

	botSvc := &stubBotSvc{}
	r := buildRouter(t, &stubProvider{}, nil, iss, botSvc, "")

	body := `{"gitflame_username":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/bot/consent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNoContent, w.Code)
}

func TestHTTP_Consent_UsernameTaken_409(t *testing.T) {
	iss := makeTestIssuer(t)

	tok, err := iss.Issue("user-uuid-002")
	require.NoError(t, err)

	botSvc := &stubBotSvc{upsertErr: botprincipal.ErrUsernameTaken}
	r := buildRouter(t, &stubProvider{}, nil, iss, botSvc, "")

	body := `{"gitflame_username":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/bot/consent", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusConflict, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "username_taken", resp["error"])
}

// --- bot-token endpoint tests ---

func TestHTTP_BotToken_EmptySecret_401(t *testing.T) {
	iss := makeTestIssuer(t)
	// Secret is "" → endpoint always rejects.
	r := buildRouter(t, &stubProvider{}, nil, iss, &stubBotSvc{found: true, userID: "u1"}, "")

	body := `{"gitflame_username":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/bot-token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", "anything")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHTTP_BotToken_WrongSecret_401(t *testing.T) {
	iss := makeTestIssuer(t)
	r := buildRouter(t, &stubProvider{}, nil, iss, &stubBotSvc{found: true, userID: "u1"}, "correct-secret")

	body := `{"gitflame_username":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/bot-token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", "wrong-secret")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestHTTP_BotToken_NotOnboarded_404(t *testing.T) {
	iss := makeTestIssuer(t)
	botSvc := &stubBotSvc{found: false}
	r := buildRouter(t, &stubProvider{}, nil, iss, botSvc, "secret123")

	body := `{"gitflame_username":"unknown"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/bot-token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", "secret123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "not_onboarded", resp["error"])
}

func TestHTTP_BotToken_Success_200(t *testing.T) {
	iss := makeTestIssuer(t)
	const userID = "user-uuid-999"
	botSvc := &stubBotSvc{found: true, userID: userID}
	r := buildRouter(t, &stubProvider{}, nil, iss, botSvc, "secret123")

	body := `{"gitflame_username":"alice"}`
	req := httptest.NewRequest(http.MethodPost, "/identity/v1/bot-token", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Service-Secret", "secret123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	accessToken, ok := resp["access_token"].(string)
	require.True(t, ok, "access_token should be a string")
	assert.NotEmpty(t, accessToken)
	assert.Equal(t, float64(1800), resp["expires_in"])

	// The minted token must verify and carry the correct sub + act_as claim.
	claims, err := iss.Verify(accessToken)
	require.NoError(t, err)
	assert.Equal(t, userID, claims.UserID)
}

func testOIDCEndpoints() transport.OIDCEndpoints {
	return transport.OIDCEndpoints{
		Authority: "https://auth.localhost/application/o/inno-agent/",
		ClientID:  "my-client-id",
	}
}
