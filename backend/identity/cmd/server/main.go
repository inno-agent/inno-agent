package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/inno-agent/identity/internal/config"
	"github.com/inno-agent/identity/internal/db"
	"github.com/inno-agent/identity/internal/delegation"
	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/provider"
	"github.com/inno-agent/identity/internal/refresh"
	"github.com/inno-agent/identity/internal/serviceclient"
	"github.com/inno-agent/identity/internal/transport"
	"github.com/inno-agent/identity/internal/user"
	"github.com/inno-agent/inno-agent/backend/pkg/telemetry"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := db.NewPool(ctx, cfg.DatabaseDSN)
	if err != nil {
		log.Fatalf("db pool: %v", err)
	}
	defer pool.Close()

	keyPEM, err := os.ReadFile(cfg.JWTPrivateKeyPath)
	if err != nil {
		log.Fatalf("read private key: %v", err)
	}

	iss, err := issuer.New(keyPEM, cfg.JWTExpiry)
	if err != nil {
		log.Fatalf("issuer: %v", err)
	}

	// Retry: on first boot the authentik worker applies the OIDC blueprint
	// after authentik-server is already healthy, so JWKS 404s briefly.
	prov, err := provider.NewOIDCProviderWithRetry(ctx, cfg.OIDCIssuer, cfg.OIDCJWKSURL, cfg.OIDCClientID, 30, 2*time.Second)
	if err != nil {
		log.Fatalf("oidc provider: %v", err)
	}

	repo := user.NewRepository(pool)
	svc := user.NewService(repo)

	refreshRepo := refresh.NewRepository(pool)
	svcClientRepo := serviceclient.NewRepository(pool)
	delegationRepo := delegation.NewRepository(pool)

	if cfg.SeedClientID != "" {
		if err := svcClientRepo.EnsureClient(ctx, cfg.SeedClientID, cfg.SeedClientSecret, cfg.SeedClientName); err != nil {
			log.Fatalf("seed service client: %v", err)
		}
	}

	telemetry.Init("identity")

	// HTTP server
	r := gin.New()
	r.Use(gin.Recovery())
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
		log.Printf("HTTP listening on :%s", cfg.HTTPPort)
		if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("HTTP serve: %v", err)
		}
	}()

	<-ctx.Done()
	log.Println("shutting down...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown: %v", err)
	}

	log.Println("done")
}
