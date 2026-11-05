// Command blog-studio runs the local-only, git-backed writing UI for nacfson_blog.
//
// Spec: docs/superpowers/specs/2026-11-04-blog-studio-design.md
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"syscall"
	"time"

	"github.com/nacfson/nacfson_blog/internal/studio"
)

func main() {
	repoFlag := flag.String("repo", ".", "Repository root (must contain content/posts)")
	addrFlag := flag.String("addr", "127.0.0.1:4321", "Loopback listen address (127.0.0.1:<port>, localhost:<port>, or [::1]:<port>)")
	branchFlag := flag.String("publish-branch", "main", "Branch Flux tracks for content")
	remoteFlag := flag.String("remote", "origin", "Git remote to fetch and push")
	openFlag := flag.Bool("open", true, "Open the browser on start")
	flag.Parse()

	if err := run(*repoFlag, *addrFlag, *branchFlag, *remoteFlag, *openFlag); err != nil {
		log.Fatalf("blog-studio: %v", err)
	}
}

func run(repo, addr, publishBranch, remote string, openBrowser bool) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hub := studio.NewHub(100)
	mgr, err := studio.NewManager(ctx, studio.Config{
		Repo:          repo,
		Remote:        remote,
		PublishBranch: publishBranch,
		Log: func(ev studio.Event) {
			hub.Publish(ev)
		},
	})
	if err != nil {
		return err
	}

	postsDir := filepath.Join(mgr.Repo(), "content", "posts")
	if fi, err := os.Stat(postsDir); err != nil || !fi.IsDir() {
		return fmt.Errorf("%s does not contain content/posts", mgr.Repo())
	}

	// Validate loopback address before taking the lock.
	if _, err := studio.NewServer(mgr, hub, addr); err != nil {
		return err
	}

	lock, err := studio.AcquireLock(mgr.Repo())
	if err != nil {
		return err
	}
	defer func() { _ = lock.Release() }()

	warnings, err := mgr.Reconcile(ctx)
	if err != nil {
		return fmt.Errorf("reconcile drafts: %w", err)
	}
	for _, w := range warnings {
		log.Printf("warning: %s", w)
	}

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	actualAddr := ln.Addr().String()
	srv, err := studio.NewServer(mgr, hub, actualAddr)
	if err != nil {
		_ = ln.Close()
		return err
	}

	httpServer := &http.Server{
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	url := "http://" + actualAddr
	log.Printf("Blog Studio listening on %s (repo: %s, branch: %s/%s)", url, mgr.Repo(), remote, publishBranch)
	if openBrowser {
		go tryOpenBrowser(url)
	}

	errCh := make(chan error, 1)
	go func() {
		if err := httpServer.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
		close(errCh)
	}()

	select {
	case <-ctx.Done():
		log.Println("shutting down Blog Studio...")
		shutCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutCtx)
	case err := <-errCh:
		return err
	}
}

func tryOpenBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}
