package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/content"
	"github.com/nacfson/nacfson_blog/internal/server"
)

func setupTestServer(t *testing.T) http.Handler {
	t.Helper()
	contentRoot, err := filepath.Abs("../../content")
	if err != nil {
		t.Fatal(err)
	}
	store, err := content.NewFolderStore(func() string { return contentRoot })
	if err != nil {
		t.Fatal(err)
	}
	webOut := t.TempDir()
	if err := os.WriteFile(filepath.Join(webOut, "index.html"), []byte("<html><body>Blog Shell</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(webOut, "post"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(webOut, "post", "index.html"), []byte("<html><body>Post Shell</body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}
	return server.NewHandler(store, func() string { return contentRoot }, webOut)
}

func TestRouter_HealthAndAPIEndpoints(t *testing.T) {
	h := setupTestServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz: expected 200, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/posts?pillar=project-updates", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/posts: expected 200, got %d", rec.Code)
	}
	var summaries []content.PostSummary
	if err := json.Unmarshal(rec.Body.Bytes(), &summaries); err != nil || len(summaries) == 0 {
		t.Fatalf("expected non-empty post list, err=%v", err)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/posts/cilium-zero-plaintext-k3s", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/api/posts/{slug}: expected 200, got %d", rec.Code)
	}
}

func TestRouter_DemoSandboxHeaders(t *testing.T) {
	h := setupTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/demos/hubble-flow-sim.html", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for demo html, got %d", rec.Code)
	}
	csp := rec.Header().Get("Content-Security-Policy")
	if !strings.Contains(csp, "connect-src 'none'") || !strings.Contains(csp, "frame-ancestors 'self'") {
		t.Errorf("missing strict CSP on demo response, got %q", csp)
	}
	if rec.Header().Get("X-Content-Type-Options") != "nosniff" {
		t.Errorf("expected X-Content-Type-Options: nosniff")
	}
}

func TestRouter_RejectsPathTraversal(t *testing.T) {
	h := setupTestServer(t)
	for _, p := range []string{
		"/content/assets/../../go.mod",
		"/content/demos/../../go.mod",
		"/api/posts/..%2fsecret",
	} {
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, p, nil))
		if rec.Code != http.StatusBadRequest && rec.Code != http.StatusNotFound {
			t.Errorf("path %q: expected 400 or 404, got %d", p, rec.Code)
		}
	}
}

func TestRouter_CompetenciesEndpoint(t *testing.T) {
	h := setupTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/competencies", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Errorf("expected application/json, got %q", ct)
	}
	var data map[string]interface{}
	if err := json.Unmarshal(rec.Body.Bytes(), &data); err != nil {
		t.Fatalf("failed to parse competencies json: %v", err)
	}
}

func TestRouter_AssetsEndpoint(t *testing.T) {
	h := setupTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/assets/hero-bg-sky.svg", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for asset, got %d", rec.Code)
	}
	if cc := rec.Header().Get("Cache-Control"); cc != "public, max-age=300" {
		t.Errorf("expected Cache-Control: public, max-age=300, got %q", cc)
	}
}

func TestRouter_SPAServingAndFallbacks(t *testing.T) {
	h := setupTestServer(t)

	// Root path should serve index.html
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Blog Shell") {
		t.Errorf("expected Blog Shell in root response, got %s", rec.Body.String())
	}

	// /post route should serve post/index.html
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/post", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /post, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Post Shell") {
		t.Errorf("expected Post Shell in /post response, got %s", rec.Body.String())
	}

	// /post/some-slug should serve post/index.html
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/post/cilium-zero-plaintext-k3s", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /post/slug, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Post Shell") {
		t.Errorf("expected Post Shell in /post/slug response, got %s", rec.Body.String())
	}

	// Other route should fall back to index.html
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/archive", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /archive fallback, got %d", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Blog Shell") {
		t.Errorf("expected Blog Shell in /archive fallback, got %s", rec.Body.String())
	}
}

func TestRouter_NotFoundAndInvalidRequests(t *testing.T) {
	h := setupTestServer(t)

	// Missing post
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/posts/non-existent-slug-1234", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing post, got %d", rec.Code)
	}

	// Missing asset
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/assets/missing-asset.svg", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing asset, got %d", rec.Code)
	}

	// Missing demo
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/demos/missing-demo.html", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for missing demo, got %d", rec.Code)
	}

	// Unmatched /api path should return 404 instead of falling back to SPA index.html
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/nonexistent", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for /api/nonexistent, got %d", rec.Code)
	}

	// Unmatched /content path should return 404 instead of falling back to SPA index.html
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/content/assets/nonexistent.svg", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("expected 404 for /content/assets/nonexistent.svg, got %d", rec.Code)
	}
}
