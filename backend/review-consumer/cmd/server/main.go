package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/config"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/gitflame"
	konsumer "github.com/inno-agent/inno-agent/backend/review-consumer/internal/kafka"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/llm"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/processor"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/review"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/tokensource"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Select token source: identity-based (production) or static (local dev).
	var tokenSrc domain.TokenSource
	if cfg.BotTokenSecret != "" {
		logger.Info(
			"using identity token source",
			zap.String("identity_url", cfg.IdentityURL),
			zap.String("bot_gitflame_username", cfg.BotGitFlameUsername),
		)
		tokenSrc = tokensource.NewIdentity(cfg.IdentityURL, cfg.BotTokenSecret, nil)
	} else {
		if cfg.OrchestratorToken == "" {
			logger.Warn("ORCHESTRATOR_TOKEN is empty and BOT_TOKEN_SECRET is not set; LLM calls will be unauthenticated and likely return 401")
		}

		tokenSrc = tokensource.NewStatic(cfg.OrchestratorToken)
	}

	gitFlameClient := gitflame.NewClient(cfg.GitFlameBaseURL, cfg.GitFlameToken)
	llmClient := llm.NewOrchestratorClient(cfg.OrchestratorURL)
	reviewService := review.NewService(gitFlameClient, llmClient, tokenSrc, cfg.ReviewModel, logger)
	proc := processor.New(reviewService, gitFlameClient, logger, cfg.BotGitFlameUsername, cfg.OnboardingURL)
	consumer := konsumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup, proc, logger)

	if err := consumer.Run(ctx); err != nil {
		logger.Fatal("consumer error", zap.Error(err))
	}
}
