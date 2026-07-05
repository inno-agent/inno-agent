package main

import (
	"context"
	"errors"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	"github.com/inno-agent/inno-agent/backend/chat-api/internal/config"
	"github.com/inno-agent/inno-agent/backend/chat-api/internal/handler"
	"github.com/inno-agent/inno-agent/backend/chat-api/internal/llm"
	"github.com/inno-agent/inno-agent/backend/chat-api/internal/repository"
	"github.com/inno-agent/inno-agent/backend/chat-api/internal/service"
	"github.com/inno-agent/inno-agent/backend/pkg/telemetry"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	logger, _ := zap.NewProduction()
	defer func() { _ = logger.Sync() }()

	if cfg.DatabaseURL == "" {
		logger.Fatal("DATABASE_URL is required")
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Fatal("failed to create db pool", zap.Error(err))
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Fatal("database not reachable", zap.Error(err))
	}

	chatRepo := repository.NewChatRepo(pool)
	messageRepo := repository.NewMessageRepo(pool)
	llmClient := llm.NewOrchestratorClient(cfg.OrchestratorURL)

	chatService := service.NewChatService(chatRepo, messageRepo, llmClient)

	chatHandler := handler.NewChatHandler(chatService)
	messageHandler := handler.NewMessageHandler(chatService)
	streamHandler := handler.NewStreamHandler(chatService)

	telemetry.Init("chat-api")

	router := chi.NewRouter()
	router.Use(telemetry.ChiMiddleware("chat-api"))
	handler.RegisterRoutes(router, chatHandler, messageHandler, streamHandler, cfg.AuthServiceURL, logger)
	router.Handle("/metrics", telemetry.Handler())

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
