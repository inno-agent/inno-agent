package main

import (
    "context"
    "net/http"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/joho/godotenv"
    "go.uber.org/zap"

    "github.com/inno-agent/inno-agent/backend/chat-api/internal/config"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/handler"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/repository"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/service"
)

func main() {
    _ = godotenv.Load()

    cfg := config.Load()

    logger, _ := zap.NewProduction()
    defer func() { _ = logger.Sync() }()

    if cfg.DatabaseURL == "" {
        logger.Fatal("DATABASE_URL is required")
    }

    ctx := context.Background()

    pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
    if err != nil {
        logger.Fatal("failed to connect to database", zap.Error(err))
    }
    defer pool.Close()

    chatRepo := repository.NewChatRepo(pool, logger)
    messageRepo := repository.NewMessageRepo(pool, logger)

    chatService := service.NewChatService(chatRepo, messageRepo, logger)

    chatHandler := handler.NewChatHandler(chatService, logger)
    messageHandler := handler.NewMessageHandler(chatService, logger)
    streamHandler := handler.NewStreamHandler(chatService, logger)

    router := chi.NewRouter()
    handler.RegisterRoutes(router, chatHandler, messageHandler, streamHandler)

    server := &http.Server{
        Addr:         ":" + cfg.ServerPort,
        Handler:      router,
        ReadTimeout:  cfg.ReadTimeout,
        WriteTimeout: cfg.WriteTimeout,
        IdleTimeout:  cfg.IdleTimeout,
    }

    logger.Info("server starting", zap.String("port", cfg.ServerPort))
    _ = server.ListenAndServe()
}