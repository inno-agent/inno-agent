package config

import (
	"os"
	"testing"
)

func TestConfig_Load_Defaults(t *testing.T) {
	envVars := []string{"SERVER_PORT", "KAFKA_BROKERS", "KAFKA_TOPIC", "WEBHOOK_AUTHORIZATION"}
	for _, v := range envVars {
		os.Unsetenv(v)
	}

	cfg := Load()

	if cfg.ServerPort != "8002" {
		t.Fatalf("expected '8002', got %q", cfg.ServerPort)
	}
	if cfg.KafkaBrokers != "redpanda:9092" {
		t.Fatalf("expected 'redpanda:9092', got %q", cfg.KafkaBrokers)
	}
	if cfg.KafkaTopic != "gitflame.events" {
		t.Fatalf("expected 'gitflame.events', got %q", cfg.KafkaTopic)
	}
}

func TestConfig_Load_CustomValues(t *testing.T) {
	t.Setenv("SERVER_PORT", "9000")
	t.Setenv("KAFKA_BROKERS", "custom:9092")

	cfg := Load()

	if cfg.ServerPort != "9000" {
		t.Fatalf("expected '9000', got %q", cfg.ServerPort)
	}
	if cfg.KafkaBrokers != "custom:9092" {
		t.Fatalf("expected 'custom:9092', got %q", cfg.KafkaBrokers)
	}
}
