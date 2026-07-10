package config

import (
	"testing"
	"time"
)

func TestConfig_Load_Defaults(t *testing.T) {
	cfg := Load()

	if cfg.ServerPort != "8001" {
		t.Fatalf("expected '8001', got %q", cfg.ServerPort)
	}
	if cfg.OrchestratorURL != "http://orchestrator:8080" {
		t.Fatalf("expected 'http://orchestrator:8080', got %q", cfg.OrchestratorURL)
	}
	if cfg.ReadTimeout != 10*time.Second {
		t.Fatalf("expected 10s, got %v", cfg.ReadTimeout)
	}
	if cfg.IdleTimeout != 120*time.Second {
		t.Fatalf("expected 120s, got %v", cfg.IdleTimeout)
	}
}

func TestConfig_Load_CustomValues(t *testing.T) {
	t.Setenv("SERVER_PORT", "9000")
	t.Setenv("ORCHESTRATOR_URL", "http://custom:8080")
	t.Setenv("READ_TIMEOUT", "30s")

	cfg := Load()

	if cfg.ServerPort != "9000" {
		t.Fatalf("expected '9000', got %q", cfg.ServerPort)
	}
	if cfg.OrchestratorURL != "http://custom:8080" {
		t.Fatalf("expected 'http://custom:8080', got %q", cfg.OrchestratorURL)
	}
	if cfg.ReadTimeout != 30*time.Second {
		t.Fatalf("expected 30s, got %v", cfg.ReadTimeout)
	}
}

func TestConfig_Load_AllowEmpty(t *testing.T) {
	// getEnvAllowEmpty: empty string is valid (not fallback)
	t.Setenv("AUTH_SERVICE_URL", "")
	cfg := Load()
	if cfg.AuthServiceURL != "" {
		t.Fatalf("expected empty AuthServiceURL, got %q", cfg.AuthServiceURL)
	}
}
