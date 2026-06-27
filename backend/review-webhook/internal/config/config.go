package config

import "os"

type Config struct {
	ServerPort            string
	KafkaBrokers          string
	KafkaTopic            string
	WebhookAuthorization  string
	WebhookAuthHeader     string
	WebhookEventHeader    string
	WebhookDeliveryHeader string
}

func Load() *Config {
	return &Config{
		ServerPort:            getEnv("SERVER_PORT", "8002"),
		KafkaBrokers:          getEnv("KAFKA_BROKERS", "redpanda:9092"),
		KafkaTopic:            getEnv("KAFKA_TOPIC", "gitflame.events"),
		WebhookAuthorization:  getEnvAllowEmpty("WEBHOOK_AUTHORIZATION", ""),
		WebhookAuthHeader:     getEnv("WEBHOOK_AUTH_HEADER", "Authorization"),
		WebhookEventHeader:    getEnv("WEBHOOK_EVENT_HEADER", "X-GitFlame-Event"),
		WebhookDeliveryHeader: getEnv("WEBHOOK_DELIVERY_HEADER", "X-GitFlame-Delivery"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// getEnvAllowEmpty uses the fallback only when the variable is unset, not when empty.
func getEnvAllowEmpty(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	return fallback
}
