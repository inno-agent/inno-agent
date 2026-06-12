package provider_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/inno-agent/identity/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	testKID      = "test-key-1"
	testClientID = "test-client"
)

func makeRSAKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	return key
}

func makeJWKSServer(t *testing.T, key *rsa.PrivateKey, issuer string) *httptest.Server {
	t.Helper()
	pub := &key.PublicKey
	jwks := map[string]interface{}{
		"keys": []map[string]interface{}{
			{
				"kty": "RSA",
				"kid": testKID,
				"alg": "RS256",
				"use": "sig",
				"n":   base64.RawURLEncoding.EncodeToString(pub.N.Bytes()),
				"e":   base64.RawURLEncoding.EncodeToString(big.NewInt(int64(pub.E)).Bytes()),
			},
		},
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func makeToken(t *testing.T, key *rsa.PrivateKey, issuer, sub, email string, exp time.Time) string {
	t.Helper()
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"sub":   sub,
		"email": email,
		"iss":   issuer,
		"aud":   jwt.ClaimStrings{testClientID},
		"exp":   jwt.NewNumericDate(exp),
		"iat":   jwt.NewNumericDate(time.Now()),
	})
	tok.Header["kid"] = testKID
	signed, err := tok.SignedString(key)
	require.NoError(t, err)
	return signed
}

func TestOIDCProvider_ValidToken(t *testing.T) {
	key := makeRSAKey(t)
	srv := makeJWKSServer(t, key, "")
	issuerURL := srv.URL

	p, err := provider.NewOIDCProvider(context.Background(), issuerURL, issuerURL+"/oauth/v2/keys", testClientID)
	require.NoError(t, err)

	token := makeToken(t, key, issuerURL, "user-123", "alice@example.com", time.Now().Add(time.Hour))
	identity, err := p.Validate(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "authentik", identity.Provider)
	assert.Equal(t, "user-123", identity.Sub)
	assert.Equal(t, "alice@example.com", identity.Email)
}

func TestOIDCProvider_ExpiredToken(t *testing.T) {
	key := makeRSAKey(t)
	srv := makeJWKSServer(t, key, "")

	p, err := provider.NewOIDCProvider(context.Background(), srv.URL, srv.URL+"/oauth/v2/keys", testClientID)
	require.NoError(t, err)

	token := makeToken(t, key, srv.URL, "user-123", "alice@example.com", time.Now().Add(-time.Hour))
	_, err = p.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestOIDCProvider_WrongIssuer(t *testing.T) {
	key := makeRSAKey(t)
	srv := makeJWKSServer(t, key, "")

	p, err := provider.NewOIDCProvider(context.Background(), srv.URL, srv.URL+"/oauth/v2/keys", testClientID)
	require.NoError(t, err)

	token := makeToken(t, key, "https://evil.example.com", "user-123", "alice@example.com", time.Now().Add(time.Hour))
	_, err = p.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestOIDCProvider_WrongAudience(t *testing.T) {
	key := makeRSAKey(t)
	srv := makeJWKSServer(t, key, "")

	p, err := provider.NewOIDCProvider(context.Background(), srv.URL, srv.URL+"/oauth/v2/keys", "other-client")
	require.NoError(t, err)

	token := makeToken(t, key, srv.URL, "user-123", "alice@example.com", time.Now().Add(time.Hour))
	_, err = p.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestNewOIDCProviderWithRetry_SucceedsAfter404s(t *testing.T) {
	key := makeRSAKey(t)
	jwksSrv := makeJWKSServer(t, key, "")

	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			http.NotFound(w, r)
			return
		}
		resp, err := http.Get(jwksSrv.URL)
		require.NoError(t, err)
		defer func() { _ = resp.Body.Close() }()
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.Copy(w, resp.Body)
	}))
	t.Cleanup(srv.Close)

	p, err := provider.NewOIDCProviderWithRetry(context.Background(), srv.URL, srv.URL+"/jwks", testClientID, 5, 10*time.Millisecond)
	require.NoError(t, err)
	require.NotNil(t, p)
	assert.GreaterOrEqual(t, atomic.LoadInt32(&calls), int32(3))
}

func TestNewOIDCProviderWithRetry_GivesUp(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)

	_, err := provider.NewOIDCProviderWithRetry(context.Background(), srv.URL, srv.URL+"/jwks", testClientID, 2, 10*time.Millisecond)
	require.Error(t, err)
}
