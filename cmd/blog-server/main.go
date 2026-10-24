package main

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/nacfson/nacfson_blog/internal/content"
	"github.com/nacfson/nacfson_blog/internal/server"
	"github.com/nacfson/nacfson_blog/internal/syncer"
)

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
	port := getEnv("PORT", "8080")
	contentWorkspaceDir := getEnv("CONTENT_WORKSPACE_DIR", "/var/run/blog-content")
	seedContentDir := getEnv("SEED_CONTENT_DIR", "/app/seed-content")
	webOutDir := getEnv("WEB_OUT_DIR", "/app/web/out")
	fluxArtifactURL := os.Getenv("FLUX_ARTIFACT_URL")

	log.Printf("initializing blog-server (port: %s, workspace: %s, seed: %s, webOut: %s)",
		port, contentWorkspaceDir, seedContentDir, webOutDir)

	reloader := &storeReloader{}
	fluxSyncer := syncer.NewFluxTarballSyncer(fluxArtifactURL, contentWorkspaceDir, reloader)

	// Seed workspace directory if not already populated
	if err := fluxSyncer.SeedIfEmpty(seedContentDir); err != nil {
		log.Fatalf("failed to seed content from %s: %v", seedContentDir, err)
	}

	// Initialize folder store with active content directory provider
	store, err := content.NewFolderStore(fluxSyncer.ActiveContentDir)
	if err != nil {
		log.Fatalf("failed to initialize content folder store: %v", err)
	}

	reloader.mu.Lock()
	reloader.store = store
	reloader.mu.Unlock()

	// Handle graceful shutdown on SIGINT or SIGTERM
	serverCtx, serverStop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer serverStop()

	// If FLUX_ARTIFACT_URL is set, run SyncOnce on startup and start periodic polling
	if fluxArtifactURL != "" {
		log.Printf("FLUX_ARTIFACT_URL set to %s; performing initial sync", fluxArtifactURL)
		func() {
			syncCtx, syncCancel := context.WithTimeout(serverCtx, 30*time.Second)
			defer syncCancel()
			updated, err := fluxSyncer.SyncOnce(syncCtx)
			if err != nil {
				log.Printf("warning: initial artifact sync failed, running with current content: %v", err)
			} else if updated {
				log.Printf("initial artifact sync successful, content updated")
			}
		}()

		fluxSyncer.Start(serverCtx, 30*time.Second)
	}

	// Build HTTP handler
	handler := server.NewHandler(store, fluxSyncer.ActiveContentDir, webOutDir)

	httpServer := &http.Server{
		Addr:         ":" + port,
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		log.Printf("blog-server listening on port %s", port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server error: %v", err)
		}
	}()

	<-serverCtx.Done()
	log.Println("shutdown signal received; stopping blog-server...")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("error during server shutdown: %v", err)
	}

	log.Println("blog-server stopped gracefully")
}
