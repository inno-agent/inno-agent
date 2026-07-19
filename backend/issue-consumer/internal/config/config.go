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

	// CodegenAgentURL is the URL of the Mastra codegen agent service.
	// If empty, falls back to single-shot LLM via OrchestratorURL.
	//
	// SECURITY / AUDIT NOTE: enabling this changes the token model for the LLM
	// call. The single-shot fallback (internal/generator) exchanges the issue
	// assigner's GitFlame identity for a delegated RFC 8693 user token via
	// internal/tokensource and attaches it to every orchestrator /v1/chat call,
	// giving per-user attribution/quota on generation. The Mastra agent calls
	// Ollama directly (bypassing the orchestrator, same as the review agent)
	// with NO per-user token — see packages/review-agent/src/mastra/agents/
	// code-generator.ts for details. Do not assume the delegated-token
	// guarantee holds once this is set.
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
		OrchestratorURL:   getEnv("ORCHESTRATOR_URL", "http://orchestrator:8080"),
		OrchestratorToken: getEnv("ORCHESTRATOR_TOKEN", ""),
		CodegenModel:      getEnv("CODEGEN_MODEL", "qwen2.5-coder:1.5b"),
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
