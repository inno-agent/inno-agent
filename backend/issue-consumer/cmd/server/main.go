package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/config"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/gitflame"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/installation"
	konsumer "github.com/inno-agent/inno-agent/backend/issue-consumer/internal/kafka"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/mastra"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/processor"
	"github.com/inno-agent/inno-agent/backend/issue-consumer/internal/tokensource"
	"github.com/inno-agent/inno-agent/backend/pkg/telemetry"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	var tokenSrc domain.TokenSource
	if cfg.ReviewDatabaseDSN != "" {
		pool, err := pgxpool.New(ctx, cfg.ReviewDatabaseDSN)
		if err != nil {
			logger.Fatal("review db pool", zap.Error(err))
		}
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
			logger.Fatal("review db ping", zap.Error(err))
		}

		if cfg.ReviewServiceClientSecret == "" {
			logger.Fatal("REVIEW_SERVICE_CLIENT_SECRET is required when REVIEW_DATABASE_DSN is set")
		}

		store := installation.NewRepository(pool)
		tokenSrc = tokensource.NewService(store, cfg.IdentityURL,
			cfg.ReviewServiceClientID, cfg.ReviewServiceClientSecret)

		logger.Info(
			"using service token source",
			zap.String("identity_url", cfg.IdentityURL),
			zap.String("client_id", cfg.ReviewServiceClientID),
		)
	} else {
		if cfg.OrchestratorToken == "" {
			logger.Warn("ORCHESTRATOR_TOKEN is empty and REVIEW_DATABASE_DSN is not set; LLM calls will be unauthenticated")
		}
		tokenSrc = tokensource.NewStatic(cfg.OrchestratorToken)
	}

	telemetry.ListenAndServe(":9090", "issue-consumer")

	gitFlameClient := gitflame.NewClient(cfg.GitFlameBaseURL, cfg.GitFlameToken)

	if cfg.CodegenAgentURL == "" {
		logger.Fatal("CODEGEN_AGENT_URL is required (the single-shot generator has been removed)")
	}
	mastraClient := mastra.NewClient(cfg.CodegenAgentURL, cfg.CodegenAgentToken)
	genService := mastra.NewGenerator(mastraClient, tokenSrc, logger)
	logger.Info("using Mastra codegen agent with per-user LLM token attribution",
		zap.String("url", cfg.CodegenAgentURL))

	proc := processor.New(genService, gitFlameClient, gitFlameClient, gitFlameClient, logger,
		cfg.BotGitFlameUsername, cfg.OnboardingURL)
	consumer := konsumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup, proc, logger)

	if err := consumer.Run(ctx); err != nil {
		logger.Fatal("consumer error", zap.Error(err))
	}
}
