package config_test

import (
	"testing"
	"time"

	"github.com/inno-agent/identity/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoad_MissingRequired(t *testing.T) {
	get := func(key string) string { return "" }
	_, err := config.LoadFrom(get)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "missing required env vars")
}

func TestLoad_AllSet(t *testing.T) {
	env := map[string]string{ //nolint:gosec
		"OIDC_ISSUER":               "https://localhost:8080/application/o/inno-agent/",
		"OIDC_CLIENT_ID":            "test-client",
		"AUTH_JWT_PRIVATE_KEY_PATH": "/tmp/key.pem",
		"AUTH_DATABASE_DSN":         "postgresql://postgres:postgres@localhost:5432/inno_auth", //nolint:gosec
		"AUTH_JWT_EXPIRY":           "15m",
		"AUTH_HTTP_PORT":            "8082",
	}
	get := func(key string) string { return env[key] }
	cfg, err := config.LoadFrom(get)
	require.NoError(t, err)
	assert.Equal(t, "https://localhost:8080/application/o/inno-agent/", cfg.OIDCIssuer)
	assert.Equal(t, "test-client", cfg.OIDCClientID)
	assert.Equal(t, 15*time.Minute, cfg.JWTExpiry)
	assert.Equal(t, "8082", cfg.HTTPPort)
}

func TestLoad_DefaultPortsAndJWKS(t *testing.T) {
	env := map[string]string{ //nolint:gosec
		"OIDC_ISSUER":               "https://localhost:8080/application/o/inno-agent/",
		"OIDC_CLIENT_ID":            "test-client",
		"AUTH_JWT_PRIVATE_KEY_PATH": "/tmp/key.pem",
		"AUTH_DATABASE_DSN":         "postgresql://localhost/inno_auth",
	}
	get := func(key string) string { return env[key] }
	cfg, err := config.LoadFrom(get)
	require.NoError(t, err)
	assert.Equal(t, "8081", cfg.HTTPPort)
	assert.Equal(t, 30*time.Minute, cfg.JWTExpiry)
	// JWKS defaults to issuer + "jwks/" (no double slash).
	assert.Equal(t, "https://localhost:8080/application/o/inno-agent/jwks/", cfg.OIDCJWKSURL)
}
