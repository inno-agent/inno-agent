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

	// ReviewAgentURL is the URL of the Mastra review agent service.
	// If empty, falls back to single-shot LLM via OrchestratorURL.
	ReviewAgentURL string
	// ReviewAgentToken is the shared secret for authenticating to the review agent.
	ReviewAgentToken string

	// Delegated-auth via identity service.
	BotGitFlameUsername string
	IdentityURL         string
	OnboardingURL       string

	// ReviewDatabaseDSN is the DSN for the shared inno_review database
	// (read installations, write rotated refresh tokens).
	ReviewDatabaseDSN string
	// ReviewServiceClientID is the client ID for service token requests.
	ReviewServiceClientID string
	// ReviewServiceClientSecret is the client secret for service token requests.
	ReviewServiceClientSecret string
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

		ReviewAgentURL:   getEnv("REVIEW_AGENT_URL", ""),
		ReviewAgentToken: getEnv("REVIEW_AGENT_AUTH_TOKEN", ""),

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
