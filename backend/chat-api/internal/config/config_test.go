package config

import (
	"testing"
	"time"
)

func TestConfig_Load_Defaults(t *testing.T) {
	cfg := Load()

	if cfg.ServerPort != "8000" {
		t.Fatalf("expected '8000', got %q", cfg.ServerPort)
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
	t.Setenv("PERF_LOG", "true")

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
	if !cfg.PerfLog {
		t.Fatal("expected PerfLog to be true")
	}
}
