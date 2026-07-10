package config

import (
	"os"
	"testing"
)

func TestConfig_Load_Defaults(t *testing.T) {
	// Clear all env vars
	envVars := []string{
		"KAFKA_BROKERS", "KAFKA_TOPIC", "KAFKA_GROUP",
		"ORCHESTRATOR_URL", "ORCHESTRATOR_TOKEN", "REVIEW_MODEL",
		"GITFLAME_BASE_URL", "GITFLAME_TOKEN",
		"BOT_GITFLAME_USERNAME", "IDENTITY_URL", "ONBOARDING_URL",
	}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg := Load()

	if cfg.KafkaBrokers != "redpanda:9092" {
		t.Fatalf("expected 'redpanda:9092', got %q", cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "gitflame.events" {
		t.Fatalf("expected 'gitflame.events', got %q", cfg.KafkaTopic)
	}
	if cfg.KafkaGroup != "review-consumer" {
		t.Fatalf("expected 'review-consumer', got %q", cfg.KafkaGroup)
	}
	if cfg.OrchestratorURL != "http://orchestrator:8080" {
		t.Fatalf("expected 'http://orchestrator:8080', got %q", cfg.OrchestratorURL)
	}
	if cfg.IdentityURL != "http://identity:8081" {
		t.Fatalf("expected 'http://identity:8081', got %q", cfg.IdentityURL)
	}
}

func TestConfig_Load_CustomValues(t *testing.T) {
	t.Setenv("KAFKA_BROKERS", "custom:9092")
	t.Setenv("REVIEW_MODEL", "qwen2.5-coder:32b")

	cfg := Load()

	if cfg.KafkaBrokers != "custom:9092" {
		t.Fatalf("expected 'custom:9092', got %q", cfg.KafkaBrokers)
	}
	if cfg.ReviewModel != "qwen2.5-coder:32b" {
		t.Fatalf("expected 'qwen2.5-coder:32b', got %q", cfg.ReviewModel)
	}
}
