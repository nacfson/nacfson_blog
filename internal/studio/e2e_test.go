package studio

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestStudio_EndToEndPlaywright(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping Playwright E2E in -short mode")
	}
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not installed")
	}

	// Ensure the embedded SPA assets exist in internal/studio/ui/dist/index.html.
	repoRoot, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	indexHTML := filepath.Join(repoRoot, "internal", "studio", "ui", "dist", "index.html")
	if _, err := os.Stat(indexHTML); err != nil {
		t.Skipf("built SPA not found at %s (run `make studio` first)", indexHTML)
	}

	e := newEnv(t)
	hub := NewHub(100)
	e.m.git.log = func(ev Event) { hub.Publish(ev) }

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	addr := ln.Addr().String()

	// Load the real built SPA from disk so the test exercises the actual bundle.
	srv, err := NewServer(e.m, hub, addr)
	if err != nil {
		t.Fatal(err)
	}
	srv.ui = os.DirFS(filepath.Join(repoRoot, "internal", "studio", "ui", "dist"))

	httpSrv := &http.Server{Handler: srv.Handler()}
	go func() { _ = httpSrv.Serve(ln) }()
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = httpSrv.Shutdown(ctx)
	}()

	shot := filepath.Join(t.TempDir(), "studio-e2e.png")
	script := filepath.Join("testdata", "e2e_playwright.mjs")
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "node", script, "http://"+addr, shot)
	out, err := cmd.CombinedOutput()
	if err != nil {
		if strings.Contains(string(out), "Playwright module not found") {
			t.Skipf("playwright not available: %s", out)
		}
		t.Fatalf("playwright E2E failed: %v\n%s", err, out)
	}

	// Verify screenshot was written.
	if fi, err := os.Stat(shot); err != nil || fi.Size() == 0 {
		t.Fatalf("expected non-empty screenshot at %s: %v", shot, err)
	}

	// Verify bare origin's main branch received exactly one `content: publish e2e-studio-post`
	// commit touching only `content/posts/2026-11-04-e2e-studio-post.md`.
	log := e.originLog(t)
	if len(log) != 2 || log[0] != "content: publish e2e-studio-post" || log[1] != "seed" {
		t.Fatalf("origin log = %v, want [content: publish e2e-studio-post, seed]", log)
	}
	files := git(t, e.origin, "show", "--name-only", "--format=", "main")
	if files != "content/posts/2026-11-04-e2e-studio-post.md" {
		t.Fatalf("published commit files = %q, want content/posts/2026-11-04-e2e-studio-post.md", files)
	}
}
