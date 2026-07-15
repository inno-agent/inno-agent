package config

import "os"

type Config struct {
	KafkaBrokers      string
	KafkaTopic        string
	KafkaGroup        string
	OrchestratorURL   string
	OrchestratorToken string
	CodegenModel      string
	GitFlameBaseURL   string
	GitFlameToken     string

	BotGitFlameUsername string
	IdentityURL         string
	OnboardingURL       string

	ReviewDatabaseDSN         string
	ReviewServiceClientID     string
	ReviewServiceClientSecret string
}

func Load() *Config {
	return &Config{
		KafkaBrokers:      getEnv("KAFKA_BROKERS", "redpanda:9092"),
		KafkaTopic:        getEnv("KAFKA_TOPIC", "gitflame.events"),
		KafkaGroup:        getEnv("KAFKA_GROUP", "issue-consumer"),
		OrchestratorURL:   getEnv("ORCHESTRATOR_URL", "http://orchestrator:8080"),
		OrchestratorToken: getEnv("ORCHESTRATOR_TOKEN", ""),
		CodegenModel:      getEnv("CODEGEN_MODEL", "qwen2.5-coder:1.5b"),
		GitFlameBaseURL:   getEnv("GITFLAME_BASE_URL", ""),
		GitFlameToken:     getEnv("GITFLAME_TOKEN", ""),

		BotGitFlameUsername: getEnv("BOT_GITFLAME_USERNAME", ""),
		IdentityURL:         getEnv("IDENTITY_URL", "http://identity:8081"),
		OnboardingURL:       getEnv("ONBOARDING_URL", ""),

		ReviewDatabaseDSN:         getEnv("REVIEW_DATABASE_DSN", ""),
		ReviewServiceClientID:     getEnv("REVIEW_SERVICE_CLIENT_ID", "review-consumer"),
		ReviewServiceClientSecret: os.Getenv("REVIEW_SERVICE_CLIENT_SECRET"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
