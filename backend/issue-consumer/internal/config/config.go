package config

import "os"

type Config struct {
	KafkaBrokers      string
	KafkaTopic        string
	KafkaGroup        string
	OrchestratorToken string
	GitFlameBaseURL   string
	GitFlameToken     string

	// CodegenAgentURL is the URL of the Mastra codegen agent service. Required —
	// the single-shot generator has been removed.
	//
	// TOKEN MODEL: internal/mastra.Generator exchanges the issue assigner's
	// GitFlame identity for a delegated RFC 8693 user token via
	// internal/tokensource (which also gates on onboarding: ErrNotOnboarded for
	// an unregistered assigner) and sends it to the codegen agent in the
	// X-Delegated-Token header; the agent forwards it to the orchestrator's
	// /v1/chat/completions as the bearer, giving per-user attribution and
	// enforcing the onboarding gate. See packages/review-agent/src/mastra/model.ts.
	CodegenAgentURL string
	// CodegenAgentToken is the shared secret for authenticating to the codegen agent.
	CodegenAgentToken string

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
		OrchestratorToken: getEnv("ORCHESTRATOR_TOKEN", ""),
		GitFlameBaseURL:   getEnv("GITFLAME_BASE_URL", ""),
		GitFlameToken:     getEnv("GITFLAME_TOKEN", ""),

		CodegenAgentURL:   getEnv("CODEGEN_AGENT_URL", ""),
		CodegenAgentToken: getEnv("CODEGEN_AGENT_AUTH_TOKEN", ""),

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
