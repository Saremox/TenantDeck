// Command tenantdeck runs the BFF: it serves the embedded frontend and the
// API in docs/route-allowlist.md on one origin, per docs/spec/03-architecture.md.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Saremox/TenantDeck/internal/auth"
	"github.com/Saremox/TenantDeck/internal/capsule"
	"github.com/Saremox/TenantDeck/internal/config"
	"github.com/Saremox/TenantDeck/internal/httpapi"
	"github.com/Saremox/TenantDeck/internal/session"
	"github.com/Saremox/TenantDeck/internal/webassets"
)

func main() {
	if err := run(); err != nil {
		slog.Error("tenantdeck exited", "error", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	backend := session.NewRedisBackend(session.RedisOptions{
		Addr:     cfg.Session.RedisAddr,
		Username: cfg.Session.RedisUsername,
		Password: cfg.Session.RedisPassword,
	})
	defer backend.Close()

	store, err := session.NewStore(backend, cfg.Session.EncryptionKey)
	if err != nil {
		return err
	}

	authHandler, err := auth.NewHandler(ctx, cfg, store)
	if err != nil {
		return err
	}

	capsuleClient := capsule.NewClient(cfg.Upstream.CapsuleProxyURL, nil)

	frontend, err := webassets.Handler()
	if err != nil {
		return err
	}

	router := httpapi.NewRouter(authHandler, store, capsuleClient, cfg.Insecure, frontend)

	server := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           router,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveErr := make(chan error, 1)
	go func() {
		slog.Info("tenantdeck listening", "addr", cfg.ListenAddr)
		serveErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serveErr:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		slog.Info("tenantdeck shutting down")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		return server.Shutdown(shutdownCtx)
	}
}
