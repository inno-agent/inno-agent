package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"

	"github.com/inno-agent/identity/internal/config"
	"github.com/inno-agent/identity/internal/db"
	"github.com/inno-agent/identity/internal/delegation"
	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/middleware"
	"github.com/inno-agent/identity/internal/provider"
	"github.com/inno-agent/identity/internal/refresh"
	"github.com/inno-agent/identity/internal/serviceclient"
	"github.com/inno-agent/identity/internal/transport"
	"github.com/inno-agent/identity/internal/user"
	"github.com/inno-agent/inno-agent/backend/pkg/logger"
	"github.com/inno-agent/inno-agent/backend/pkg/telemetry"
	"github.com/inno-agent/inno-agent/backend/pkg/tracing"
)

func main() {
	log := logger.New("identity")
	defer func() { _ = log.Sync() }()

	cfg, err := config.Load()
	if err != nil {
		log.Fatal("config", zap.Error(err))
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	tracingCleanup, err := tracing.Setup(ctx, "identity")
	if err != nil {
		log.Fatal("tracing init", zap.Error(err))
	}
	defer tracingCleanup()

	pool, err := db.NewPool(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Fatal("db pool", zap.Error(err))
	}
	defer pool.Close()

	keyPEM, err := os.ReadFile(cfg.JWTPrivateKeyPath)
	if err != nil {
		log.Fatal("read private key", zap.Error(err))
	}

	iss, err := issuer.New(keyPEM, cfg.JWTExpiry)
	if err != nil {
		log.Fatal("issuer", zap.Error(err))
	}

	// Retry: on first boot the authentik worker applies the OIDC blueprint
	// after authentik-server is already healthy, so JWKS 404s briefly.
	prov, err := provider.NewOIDCProviderWithRetry(ctx, cfg.OIDCIssuer, cfg.OIDCJWKSURL, cfg.OIDCClientID, 30, 2*time.Second)
	if err != nil {
		log.Fatal("oidc provider", zap.Error(err))
	}

	repo := user.NewRepository(pool)
	svc := user.NewService(repo)

	refreshRepo := refresh.NewRepository(pool)
	svcClientRepo := serviceclient.NewRepository(pool)
	delegationRepo := delegation.NewRepository(pool)

	if cfg.SeedClientID != "" {
		if err := svcClientRepo.EnsureClient(ctx, cfg.SeedClientID, cfg.SeedClientSecret, cfg.SeedClientName); err != nil {
			log.Fatal("seed service client", zap.Error(err))
		}
	}

	telemetry.Init("identity")

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(tracing.GinMiddleware("identity"))
	r.Use(middleware.Adapt(logger.CorrelationID))
	r.Use(middleware.Adapt(logger.InjectLogger(log)))
	r.Use(middleware.AccessLog())
	r.Use(telemetry.GinMiddleware("identity"))
	transport.RegisterHTTPRoutes(r, prov, svc, iss, cfg.JWTExpiry, transport.OIDCEndpoints{
		Authority: cfg.OIDCIssuer,
		ClientID:  cfg.OIDCClientID,
	}, refreshRepo, cfg.RefreshExpiry, svcClientRepo, cfg.ServiceTokenExpiry, delegationRepo, cfg.DelegateTokenExpiry)
	r.GET("/metrics", gin.WrapH(telemetry.Handler()))

	httpSrv := &http.Server{
		Addr:              ":" + cfg.HTTPPort,
		Handler:           r,
		ReadHeaderTimeout: 5 * time.Second,
	}
	go func() {
		log.Info("HTTP listening", zap.String("port", cfg.HTTPPort))
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("HTTP serve", zap.Error(err))
		}
	}()

	<-ctx.Done()
	log.Info("shutting down")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Error("HTTP shutdown", zap.Error(err))
	}

	log.Info("done")
}
