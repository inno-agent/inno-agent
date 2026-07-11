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
	"github.com/inno-agent/inno-agent/backend/review-webhook/internal/config"
	internalkafka "github.com/inno-agent/inno-agent/backend/review-webhook/internal/kafka"
	"github.com/inno-agent/inno-agent/backend/review-webhook/internal/webhook"
)

func main() {
	_ = godotenv.Load()

	cfg := config.Load()

	log := logger.New("review-webhook")
	defer func() { _ = log.Sync() }()

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer stop()

	publisher := internalkafka.NewPublisher(cfg.KafkaBrokers, cfg.KafkaTopic)
	defer func() { _ = publisher.Close() }()

	webhookHandler := webhook.New(cfg, publisher, log)

	telemetry.Init("review-webhook")

	router := chi.NewRouter()
	router.Use(logger.CorrelationID)
	router.Use(logger.InjectLogger(log))
	router.Use(logger.RequestLogger())
	router.Use(telemetry.ChiMiddleware("review-webhook"))

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
	router.Post("/hooks/gitflame", webhookHandler.ServeHTTP)
	router.Handle("/metrics", telemetry.Handler())

	server := &http.Server{
		Addr:         ":" + cfg.ServerPort,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
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
