package config_test

import (
	"testing"
	"time"

	"innoagent/internal/config"
)

func TestLoadCompletionsDefaults(t *testing.T) {
	t.Setenv("LLM_COMPLETIONS_TIMEOUT", "")
	t.Setenv("LLM_MAX_BODY_BYTES", "")

	cfg := config.Load()

	if cfg.CompletionsTimeout != 180*time.Second {
		t.Errorf("timeout = %v, want 180s", cfg.CompletionsTimeout)
	}
	if cfg.MaxBodyBytes != 10<<20 {
		t.Errorf("maxBodyBytes = %d, want %d", cfg.MaxBodyBytes, 10<<20)
	}
}

func TestLoadCompletionsOverrides(t *testing.T) {
	t.Setenv("LLM_COMPLETIONS_TIMEOUT", "45s")
	t.Setenv("LLM_MAX_BODY_BYTES", "1024")

	cfg := config.Load()

	if cfg.CompletionsTimeout != 45*time.Second {
		t.Errorf("timeout = %v, want 45s", cfg.CompletionsTimeout)
	}
	if cfg.MaxBodyBytes != 1024 {
		t.Errorf("maxBodyBytes = %d, want 1024", cfg.MaxBodyBytes)
	}
}

func TestLoadCompletionsIgnoresGarbage(t *testing.T) {
	tests := []struct {
		name    string
		timeout string
		maxBody string
	}{
		{"unparseable", "not-a-duration", "not-a-number"},
		{"negative", "-30s", "-5"},
		{"zero", "0s", "0"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("LLM_COMPLETIONS_TIMEOUT", tt.timeout)
			t.Setenv("LLM_MAX_BODY_BYTES", tt.maxBody)

			cfg := config.Load()

			if cfg.CompletionsTimeout != 180*time.Second {
				t.Errorf("timeout = %v, want 180s fallback", cfg.CompletionsTimeout)
			}
			if cfg.MaxBodyBytes != 10<<20 {
				t.Errorf("maxBodyBytes = %d, want default fallback", cfg.MaxBodyBytes)
			}
		})
	}
}
