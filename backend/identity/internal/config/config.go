package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	// OIDCIssuer is the public issuer URL the browser sees,
	// e.g. https://localhost:8080/application/o/inno-agent/
	OIDCIssuer string
	// OIDCJWKSURL is the internal URL to fetch the IdP signing keys from.
	OIDCJWKSURL       string
	OIDCClientID      string
	JWTPrivateKeyPath string
	JWTExpiry         time.Duration
	DatabaseDSN       string
	HTTPPort          string
	// RefreshExpiry is the TTL for refresh tokens.
	RefreshExpiry time.Duration
	// ServiceTokenExpiry is the TTL for service tokens.
	ServiceTokenExpiry time.Duration
	// DelegateTokenExpiry is the TTL for delegated tokens issued via RFC 8693 token exchange.
	DelegateTokenExpiry time.Duration
	// SeedClientID/Secret/Name: if set, identity upserts this client at startup.
	SeedClientID     string
	SeedClientSecret string
	SeedClientName   string
}

func Load() (*Config, error) {
	return LoadFrom(os.Getenv)
}

func LoadFrom(getenv func(string) string) (*Config, error) {
	var missing []string
	require := func(key string) string {
		v := getenv(key)
		if v == "" {
			missing = append(missing, key)
		}
		return v
	}
	fallback := func(key, def string) string {
		if v := getenv(key); v != "" {
			return v
		}
		return def
	}

	issuer := require("OIDC_ISSUER")
	cfg := &Config{
		OIDCIssuer:        issuer,
		OIDCJWKSURL:       fallback("OIDC_JWKS_URL", strings.TrimSuffix(issuer, "/")+"/jwks/"),
		OIDCClientID:      require("OIDC_CLIENT_ID"),
		JWTPrivateKeyPath: require("AUTH_JWT_PRIVATE_KEY_PATH"),
		DatabaseDSN:       require("AUTH_DATABASE_DSN"),
		HTTPPort:          fallback("AUTH_HTTP_PORT", "8081"),
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: [%s]", strings.Join(missing, ", "))
	}

	expiry, err := time.ParseDuration(fallback("AUTH_JWT_EXPIRY", "30m"))
	if err != nil {
		return nil, fmt.Errorf("invalid AUTH_JWT_EXPIRY: %w", err)
	}
	cfg.JWTExpiry = expiry

	refreshExpiry, err := time.ParseDuration(fallback("AUTH_REFRESH_EXPIRY", "720h"))
	if err != nil {
		return nil, fmt.Errorf("invalid AUTH_REFRESH_EXPIRY: %w", err)
	}
	cfg.RefreshExpiry = refreshExpiry

	serviceTokenExpiry, err := time.ParseDuration(fallback("SERVICE_TOKEN_EXPIRY", "1h"))
	if err != nil {
		return nil, fmt.Errorf("invalid SERVICE_TOKEN_EXPIRY: %w", err)
	}
	cfg.ServiceTokenExpiry = serviceTokenExpiry

	delegateTokenExpiry, err := time.ParseDuration(fallback("DELEGATE_TOKEN_EXPIRY", "15m"))
	if err != nil {
		return nil, fmt.Errorf("invalid DELEGATE_TOKEN_EXPIRY: %w", err)
	}
	cfg.DelegateTokenExpiry = delegateTokenExpiry

	cfg.SeedClientID = getenv("SEED_CLIENT_ID")
	cfg.SeedClientSecret = getenv("SEED_CLIENT_SECRET")
	cfg.SeedClientName = fallback("SEED_CLIENT_NAME", cfg.SeedClientID)

	return cfg, nil
}
