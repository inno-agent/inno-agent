package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/metrics"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/config"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/gitflame"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/identityclient"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/installation"
	konsumer "github.com/inno-agent/inno-agent/backend/review-consumer/internal/kafka"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/llm"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/processor"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/review"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/secretbox"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/tokensource"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	// Select token source: installation-based (production) or static (local dev).
	var tokenSrc domain.TokenSource
	if cfg.ReviewDatabaseDSN != "" && cfg.ReviewRefreshEncKey != "" {
		pool, err := pgxpool.New(ctx, cfg.ReviewDatabaseDSN)
		if err != nil {
			logger.Fatal("review db pool", zap.Error(err))
		}
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
			logger.Fatal("review db ping", zap.Error(err))
		}

		sb, err := secretbox.NewFromBase64Key(cfg.ReviewRefreshEncKey)
		if err != nil {
			logger.Fatal("secretbox", zap.Error(err))
		}

		store := installation.NewRepository(pool)
		idClient := identityclient.New(cfg.IdentityURL, nil)
		tokenSrc = tokensource.NewInstallation(store, idClient, sb)

		logger.Info(
			"using installation token source",
			zap.String("identity_url", cfg.IdentityURL),
			zap.String("bot_gitflame_username", cfg.BotGitFlameUsername),
		)
	} else {
		if cfg.OrchestratorToken == "" {
			logger.Warn("ORCHESTRATOR_TOKEN is empty and REVIEW_DATABASE_DSN/REVIEW_REFRESH_ENC_KEY are not set; LLM calls will be unauthenticated and likely return 401")
		}

		tokenSrc = tokensource.NewStatic(cfg.OrchestratorToken)
	}

	metrics.ListenAndServe(":9090", "review-consumer")

	gitFlameClient := gitflame.NewClient(cfg.GitFlameBaseURL, cfg.GitFlameToken)
	llmClient := llm.NewOrchestratorClient(cfg.OrchestratorURL)
	reviewService := review.NewService(gitFlameClient, llmClient, tokenSrc, cfg.ReviewModel, logger)
	proc := processor.New(reviewService, gitFlameClient, logger, cfg.BotGitFlameUsername, cfg.OnboardingURL)
	consumer := konsumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup, proc, logger)

	if err := consumer.Run(ctx); err != nil {
		logger.Fatal("consumer error", zap.Error(err))
	}
}
