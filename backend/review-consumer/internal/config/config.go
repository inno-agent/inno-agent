package config

import "os"

type Config struct {
	KafkaBrokers      string
	KafkaTopic        string
	KafkaGroup        string
	OrchestratorURL   string
	OrchestratorToken string
	ReviewModel       string
	GitFlameBaseURL   string
	GitFlameToken     string

	// Slice 2: delegated-auth via identity service.
	BotGitFlameUsername string
	IdentityURL         string
	BotTokenSecret      string
	OnboardingURL       string
}

func Load() *Config {
	return &Config{
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "redpanda:9092"),
		KafkaTopic:        getEnv("KAFKA_TOPIC", "gitflame.events"),
		KafkaGroup:        getEnv("KAFKA_GROUP", "review-consumer"),
		OrchestratorURL:   getEnv("ORCHESTRATOR_URL", "http://orchestrator:8080"),
		OrchestratorToken: getEnv("ORCHESTRATOR_TOKEN", ""),
		ReviewModel:       getEnv("REVIEW_MODEL", "qwen2.5-coder:1.5b"),
		GitFlameBaseURL:   getEnv("GITFLAME_BASE_URL", ""),
		GitFlameToken:     getEnv("GITFLAME_TOKEN", ""),

		BotGitFlameUsername: getEnv("BOT_GITFLAME_USERNAME", ""),
		IdentityURL:         getEnv("IDENTITY_URL", "http://identity:8081"),
		BotTokenSecret:      getEnv("BOT_TOKEN_SECRET", ""),
		OnboardingURL:       getEnv("ONBOARDING_URL", ""),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}

	return fallback
}
