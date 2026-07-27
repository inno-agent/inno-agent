package config

import "testing"

func TestLoad_ReviewAgentToken(t *testing.T) {
	t.Setenv("REVIEW_AGENT_AUTH_TOKEN", "secret-xyz")
	cfg := Load()
	if cfg.ReviewAgentToken != "secret-xyz" {
		t.Fatalf("ReviewAgentToken = %q, want %q", cfg.ReviewAgentToken, "secret-xyz")
	}
}
