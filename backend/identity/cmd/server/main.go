package main

import (
	"context"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"

	"github.com/inno-agent/identity/internal/config"
	"github.com/inno-agent/identity/internal/db"
	"github.com/inno-agent/identity/internal/issuer"
	"github.com/inno-agent/identity/internal/provider"
	"github.com/inno-agent/identity/internal/transport"
	"github.com/inno-agent/identity/internal/user"
	identityv1 "github.com/inno-agent/identity/proto/identity/v1"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := db.EnsureDatabase(ctx, cfg.DatabaseDSN); err != nil {
		log.Fatalf("ensure db: %v", err)
	}

	if err := db.Migrate(cfg.DatabaseDSN); err != nil {
		log.Fatalf("migrate: %v", err)
	}

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

	// gRPC server
	grpcSrv := grpc.NewServer()
	identityv1.RegisterIdentityServiceServer(grpcSrv, transport.NewGRPCServer(iss, svc))

	grpcLis, err := net.Listen("tcp", ":"+cfg.GRPCPort)
	if err != nil {
		log.Fatalf("grpc listen: %v", err)
	}
	go func() {
		log.Printf("gRPC listening on :%s", cfg.GRPCPort)
		if err := grpcSrv.Serve(grpcLis); err != nil {
			log.Printf("gRPC serve: %v", err)
		}
	}()

	// HTTP server
	r := gin.New()
	r.Use(gin.Recovery())
	transport.RegisterHTTPRoutes(r, prov, svc, iss, cfg.JWTExpiry, transport.OIDCEndpoints{
		Authority:    cfg.OIDCIssuer,
		AuthorizeURL: cfg.OIDCAuthorizeURL,
		ClientID:     cfg.OIDCClientID,
		TokenURL:     cfg.OIDCTokenURL,
		JWKSURL:      cfg.OIDCJWKSURL,
	})

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

	grpcSrv.GracefulStop()

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := httpSrv.Shutdown(shutdownCtx); err != nil {
		log.Printf("HTTP shutdown: %v", err)
	}

	log.Println("done")
}
