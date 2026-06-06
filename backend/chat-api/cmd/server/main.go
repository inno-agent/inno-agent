package main

import (
    "context"
    "net/http"
    "os"
    "time"

    "github.com/go-chi/chi/v5"
    "github.com/jackc/pgx/v5/pgxpool"
    "github.com/joho/godotenv"
    "go.uber.org/zap"

    "github.com/inno-agent/inno-agent/backend/chat-api/internal/handler"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/repository"
    "github.com/inno-agent/inno-agent/backend/chat-api/internal/service"
)

func main() {
    _ = godotenv.Load()

    logger, _ := zap.NewProduction()
    defer func() { _ = logger.Sync() }()

    databaseURL := os.Getenv("DATABASE_URL")
    if databaseURL == "" {
        logger.Fatal("DATABASE_URL is required")
    }

    serverPort := os.Getenv("SERVER_PORT")
    if serverPort == "" {
        serverPort = "8000"
    }

    ctx := context.Background()

    pool, err := pgxpool.New(ctx, databaseURL)
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
        Addr:         ":" + serverPort,
        Handler:      router,
        ReadTimeout:  10 * time.Second,
        WriteTimeout: 0,
        IdleTimeout:  120 * time.Second,
    }

    logger.Info("server starting", zap.String("port", serverPort))
    _ = server.ListenAndServe()
}