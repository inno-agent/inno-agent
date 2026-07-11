package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/pkg/logger"
	"github.com/inno-agent/inno-agent/backend/pkg/telemetry"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/config"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/db"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/gitflame"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/handler"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/identityclient"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/installation"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/llm"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/service"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	log := logger.New("review-api")
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	llmClient := llm.NewOrchestratorClient(cfg.OrchestratorURL)
	gitFlameClient := gitflame.NewClient(cfg.GitFlameBaseURL, cfg.GitFlameToken)

	reviewService := service.NewReviewService(gitFlameClient, llmClient)
	reviewHandler := handler.NewReviewHandler(reviewService)

	// Onboarding (installations) and invite acceptance both need the review DB —
	// invite acceptance looks up the caller's own linked gitflame_username there,
	// so the repo owner can never be spoofed via the request body.
	var installHandler *handler.InstallationHandler
	var inviteHandler *handler.InviteHandler
	if cfg.ReviewDatabaseDSN != "" {
		pool, err := db.NewPool(ctx, cfg.ReviewDatabaseDSN)
		if err != nil {
			log.Fatal("review db pool", zap.Error(err))
		}
		defer pool.Close()

		installRepo := installation.NewRepository(pool)
		identityClient := identityclient.New(cfg.AuthServiceURL)
		installHandler = handler.NewInstallationHandler(installRepo, identityClient, cfg.ReviewConsumerClientID, log)
		inviteHandler = handler.NewInviteHandler(installRepo, gitFlameClient, log)
		log.Info("onboarding enabled (installations)")
	} else {
		log.Warn("REVIEW_DATABASE_DSN unset; /installations and /invitations/accept disabled")
	}

	telemetry.Init("review-api")

	router := chi.NewRouter()
	router.Use(logger.CorrelationID)
	router.Use(logger.InjectLogger(log))
	router.Use(logger.RequestLogger())
	router.Use(telemetry.ChiMiddleware("review-api"))
	handler.RegisterRoutes(router, reviewHandler, installHandler, inviteHandler, cfg.AuthServiceURL)
	router.Handle("/metrics", telemetry.Handler())

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		log.Info("server starting", zap.String("port", cfg.ServerPort))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error("shutdown error", zap.Error(err))
	}
}
