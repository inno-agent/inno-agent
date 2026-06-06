package config_test

import (
	"testing"
	"time"

	"github.com/inno-agent/auth/internal/config"
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
	env := map[string]string{
		"ZITADEL_ISSUER":              "http://localhost:8080",
		"ZITADEL_CLIENT_ID":           "test-client",
		"AUTH_JWT_PRIVATE_KEY_PATH":   "/tmp/key.pem",
		"AUTH_DATABASE_DSN":           "postgresql://postgres:postgres@localhost:5432/inno_auth",
		"AUTH_JWT_EXPIRY":             "15m",
		"AUTH_HTTP_PORT":              "8082",
		"AUTH_GRPC_PORT":              "9092",
	}
	get := func(key string) string { return env[key] }
	cfg, err := config.LoadFrom(get)
	require.NoError(t, err)
	assert.Equal(t, "http://localhost:8080", cfg.ZitadelIssuer)
	assert.Equal(t, "test-client", cfg.ZitadelClientID)
	assert.Equal(t, 15*time.Minute, cfg.JWTExpiry)
	assert.Equal(t, "8082", cfg.HTTPPort)
}

func TestLoad_DefaultPorts(t *testing.T) {
	env := map[string]string{
		"ZITADEL_ISSUER":            "http://localhost:8080",
		"ZITADEL_CLIENT_ID":         "client",
		"AUTH_JWT_PRIVATE_KEY_PATH": "/tmp/key.pem",
		"AUTH_DATABASE_DSN":         "postgresql://localhost/inno_auth",
	}
	get := func(key string) string { return env[key] }
	cfg, err := config.LoadFrom(get)
	require.NoError(t, err)
	assert.Equal(t, "8081", cfg.HTTPPort)
	assert.Equal(t, "9091", cfg.GRPCPort)
	assert.Equal(t, 30*time.Minute, cfg.JWTExpiry)
}
