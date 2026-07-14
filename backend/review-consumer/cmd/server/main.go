package main

import (
	"context"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/pkg/logger"
	"github.com/inno-agent/inno-agent/backend/pkg/telemetry"
	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/config"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/domain"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/gitflame"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/installation"
	konsumer "github.com/inno-agent/inno-agent/backend/review-consumer/internal/kafka"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/llm"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/mastra"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/processor"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/review"
	"github.com/inno-agent/inno-agent/backend/review-consumer/internal/tokensource"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	log := logger.New("review-consumer")
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	tracingCleanup, err := tracing.Setup(ctx, "review-consumer")
	if err != nil {
		log.Fatal("tracing init", zap.Error(err))
	}
	defer tracingCleanup()

	var tokenSrc domain.TokenSource
	if cfg.ReviewDatabaseDSN != "" {
		pool, err := pgxpool.New(ctx, cfg.ReviewDatabaseDSN)
		if err != nil {
			log.Fatal("review db pool", zap.Error(err))
		}
		defer pool.Close()

		if err := pool.Ping(ctx); err != nil {
			log.Fatal("review db ping", zap.Error(err))
		}

		if cfg.ReviewServiceClientSecret == "" {
			log.Fatal("REVIEW_SERVICE_CLIENT_SECRET is required when REVIEW_DATABASE_DSN is set")
		}

		store := installation.NewRepository(pool)
		tokenSrc = tokensource.NewService(store, cfg.IdentityURL,
			cfg.ReviewServiceClientID, cfg.ReviewServiceClientSecret)

		log.Info(
			"using service token source",
			zap.String("identity_url", cfg.IdentityURL),
			zap.String("client_id", cfg.ReviewServiceClientID),
		)
	} else {
		if cfg.OrchestratorToken == "" {
			log.Warn("ORCHESTRATOR_TOKEN is empty and REVIEW_DATABASE_DSN is not set; LLM calls will be unauthenticated")
		}
		tokenSrc = tokensource.NewStatic(cfg.OrchestratorToken)
	}

	telemetry.ListenAndServe(":9090", "review-consumer")

	gitFlameClient := gitflame.NewClient(cfg.GitFlameBaseURL, cfg.GitFlameToken)

	var reviewer domain.Reviewer
	if cfg.ReviewAgentURL != "" {
		mastraClient := mastra.NewClient(cfg.ReviewAgentURL, cfg.ReviewAgentToken)
		reviewer = review.NewMastraReviewer(mastraClient, log)
		log.Info("using Mastra review agent", zap.String("url", cfg.ReviewAgentURL))
	} else {
		llmClient := llm.NewOrchestratorClient(cfg.OrchestratorURL)
		reviewer = review.NewService(gitFlameClient, llmClient, tokenSrc, cfg.ReviewModel, log)
		log.Info("using single-shot LLM review", zap.String("orchestrator", cfg.OrchestratorURL))
	}

	proc := processor.New(reviewer, gitFlameClient, log, cfg.BotGitFlameUsername, cfg.OnboardingURL)
	consumer := konsumer.NewConsumer(cfg.KafkaBrokers, cfg.KafkaTopic, cfg.KafkaGroup, proc, log)

	if err := consumer.Run(ctx); err != nil {
		log.Fatal("consumer error", zap.Error(err))
	}
}
