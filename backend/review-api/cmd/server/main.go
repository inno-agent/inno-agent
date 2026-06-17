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

	"github.com/inno-agent/inno-agent/backend/review-api/internal/config"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/gitflame"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/handler"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/llm"
	"github.com/inno-agent/inno-agent/backend/review-api/internal/service"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	llmClient := llm.NewOrchestratorClient(cfg.OrchestratorURL)
	gitFlameClient := gitflame.NewClient(cfg.GitFlameBaseURL, cfg.GitFlameToken)

	reviewService := service.NewReviewService(gitFlameClient, llmClient, logger)
	reviewHandler := handler.NewReviewHandler(reviewService, logger)

	router := chi.NewRouter()
	handler.RegisterRoutes(router, reviewHandler, cfg.AuthServiceURL)

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  cfg.IdleTimeout,
	}

	go func() {
		logger.Info("server starting", zap.String("port", cfg.ServerPort))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatal("server error", zap.Error(err))
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		logger.Error("shutdown error", zap.Error(err))
	}
}
