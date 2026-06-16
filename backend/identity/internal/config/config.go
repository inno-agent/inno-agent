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
	GRPCPort          string
	AllowedModels     []string
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
		GRPCPort:          fallback("AUTH_GRPC_PORT", "9091"),
	}

	if len(missing) > 0 {
		return nil, fmt.Errorf("missing required env vars: [%s]", strings.Join(missing, ", "))
	}

	expiry, err := time.ParseDuration(fallback("AUTH_JWT_EXPIRY", "30m"))
	if err != nil {
		return nil, fmt.Errorf("invalid AUTH_JWT_EXPIRY: %w", err)
	}
	cfg.JWTExpiry = expiry

	cfg.AllowedModels = strings.Fields(fallback("LLM_MODELS", "llama3.2:3b qwen2.5-coder:7b deepseek-r1:8b"))

	return cfg, nil
}
