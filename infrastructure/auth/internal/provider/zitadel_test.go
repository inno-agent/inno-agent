package provider_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/inno-agent/auth/internal/provider"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testKID = "test-key-1"
const testClientID = "test-client"

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
		json.NewEncoder(w).Encode(jwks)
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

func TestZitadelProvider_ValidToken(t *testing.T) {
	key := makeRSAKey(t)
	srv := makeJWKSServer(t, key, "")
	issuerURL := srv.URL

	p, err := provider.NewZitadelProvider(context.Background(), issuerURL, testClientID)
	require.NoError(t, err)

	token := makeToken(t, key, issuerURL, "user-123", "alice@example.com", time.Now().Add(time.Hour))
	identity, err := p.Validate(context.Background(), token)
	require.NoError(t, err)
	assert.Equal(t, "zitadel", identity.Provider)
	assert.Equal(t, "user-123", identity.Sub)
	assert.Equal(t, "alice@example.com", identity.Email)
}

func TestZitadelProvider_ExpiredToken(t *testing.T) {
	key := makeRSAKey(t)
	srv := makeJWKSServer(t, key, "")

	p, err := provider.NewZitadelProvider(context.Background(), srv.URL, testClientID)
	require.NoError(t, err)

	token := makeToken(t, key, srv.URL, "user-123", "alice@example.com", time.Now().Add(-time.Hour))
	_, err = p.Validate(context.Background(), token)
	require.Error(t, err)
}

func TestZitadelProvider_WrongAudience(t *testing.T) {
	key := makeRSAKey(t)
	srv := makeJWKSServer(t, key, "")

	p, err := provider.NewZitadelProvider(context.Background(), srv.URL, "other-client")
	require.NoError(t, err)

	token := makeToken(t, key, srv.URL, "user-123", "alice@example.com", time.Now().Add(time.Hour))
	_, err = p.Validate(context.Background(), token)
	require.Error(t, err)
}
