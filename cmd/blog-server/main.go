package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nacfson/nacfson_blog/internal/content"
	"github.com/nacfson/nacfson_blog/internal/logx"
	"github.com/nacfson/nacfson_blog/internal/server"
	"github.com/nacfson/nacfson_blog/internal/syncer"
)

const serviceName = "blog-web"

var version = "v0.0.0"

type storeReloader struct {
	mu    sync.Mutex
	store *content.FolderStore
}

func (r *storeReloader) Reload() error {
	r.mu.Lock()
	s := r.store
	r.mu.Unlock()
	if s != nil {
		return s.Reload()
	}
	return nil
}

func getEnv(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}

func main() {
	logger := logx.New(os.Stderr, serviceName, version)
	slog.SetDefault(logger)

	port := getEnv("PORT", "8080")
	contentWorkspaceDir := getEnv("CONTENT_WORKSPACE_DIR", "/var/run/blog-content")
	seedContentDir := getEnv("SEED_CONTENT_DIR", "/app/seed-content")
	webOutDir := getEnv("WEB_OUT_DIR", "/app/web/out")
	fluxArtifactURL := os.Getenv("FLUX_ARTIFACT_URL")

	logger.Info("initializing blog-server",
		"port", port,
		"workspace", contentWorkspaceDir,
		"seed", seedContentDir,
		"web_out", webOutDir,
	)

	reloader := &storeReloader{}
	fluxSyncer := syncer.NewFluxTarballSyncer(fluxArtifactURL, contentWorkspaceDir, reloader)

	// Seed workspace directory if not already populated
	if err := fluxSyncer.SeedIfEmpty(seedContentDir); err != nil {
		logx.Fatal(logger, "failed to seed content", err, "seed_dir", seedContentDir)
	}

	// Initialize folder store with active content directory provider
	store, err := content.NewFolderStore(fluxSyncer.ActiveContentDir)
	if err != nil {
		logx.Fatal(logger, "failed to initialize content folder store", err)
	}

	reloader.mu.Lock()
	reloader.store = store
	reloader.mu.Unlock()

	// Handle graceful shutdown on SIGINT or SIGTERM
	serverCtx, serverStop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer serverStop()

	// If FLUX_ARTIFACT_URL is set, run SyncOnce on startup and start periodic polling
	if fluxArtifactURL != "" {
		logger.Info("performing initial flux artifact sync", "flux_artifact_url", fluxArtifactURL)
		func() {
			syncCtx, syncCancel := context.WithTimeout(serverCtx, 30*time.Second)
			defer syncCancel()
			updated, err := fluxSyncer.SyncOnce(syncCtx)
			if err != nil {
				logger.Error("initial artifact sync failed, running with current content", "error", err)
			} else if updated {
				logger.Info("initial artifact sync successful, content updated")
			}
		}()

		fluxSyncer.Start(serverCtx, 30*time.Second)
	}

	// Build HTTP handler
	handler := server.NewHandler(store, fluxSyncer.ActiveContentDir, webOutDir)

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ErrorLog:     slog.NewLogLogger(logger.Handler(), slog.LevelError),
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("blog-server listening", "port", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logx.Fatal(logger, "http server error", err)
		}
	}()

	<-serverCtx.Done()
	logger.Info("shutdown signal received; stopping blog-server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		logger.Error("error during server shutdown", "error", err)
	}

	logger.Info("blog-server stopped gracefully")
}
