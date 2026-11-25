package server_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/analytics"
	"github.com/nacfson/nacfson_blog/internal/content"
	"github.com/nacfson/nacfson_blog/internal/logx"
	"github.com/nacfson/nacfson_blog/internal/server"
)

func discardViews() *analytics.Recorder {
	return analytics.New(logx.New(io.Discard, "", ""))
}

func setupTestServer(t *testing.T) http.Handler {
	t.Helper()
	return setupTestServerWithViews(t, discardViews())
}

func setupTestServerWithViews(t *testing.T, views *analytics.Recorder) http.Handler {
	t.Helper()
	contentRoot, err := filepath.Abs("../../tests/fixtures/content")
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
	return server.NewHandler(store, func() string { return contentRoot }, webOut, views)
}

func TestRouter_HealthAndAPIEndpoints(t *testing.T) {
	h := setupTestServer(t)

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("/healthz: expected 200, got %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/posts?topic=cloud-native", nil))
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

func TestRouter_CompetenciesEndpointRemoved(t *testing.T) {
	h := setupTestServer(t)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/competencies", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404 for removed /api/competencies, got %d", rec.Code)
	}
}

func TestRouter_EmptyContentServesEmptyPostList(t *testing.T) {
	contentRoot := t.TempDir()
	if err := os.MkdirAll(filepath.Join(contentRoot, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	store, err := content.NewFolderStore(func() string { return contentRoot })
	if err != nil {
		t.Fatalf("NewFolderStore on empty content: %v", err)
	}
	h := server.NewHandler(store, func() string { return contentRoot }, t.TempDir(), discardViews())

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/posts", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200 for /api/posts on empty content, got %d", rec.Code)
	}
	if body := strings.TrimSpace(rec.Body.String()); body != "[]" {
		t.Fatalf("expected empty JSON array, got %q", body)
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

type failingStore struct {
	err error
}

func (f *failingStore) ListPosts(ctx context.Context, topic string) ([]content.PostSummary, error) {
	return nil, f.err
}

func (f *failingStore) GetPost(ctx context.Context, slug string) (*content.Post, error) {
	return nil, f.err
}

func TestRouter_RequestLoggingAndTraceparent(t *testing.T) {
	var buf bytes.Buffer
	prev := slog.Default()
	t.Cleanup(func() { slog.SetDefault(prev) })
	slog.SetDefault(logx.New(&buf, "blog-web", "v1.4.2"))

	h := setupTestServer(t)

	// 1. Request with valid W3C traceparent
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", rec.Code)
	}

	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("expected JSON access log line, got %q (err=%v)", buf.String(), err)
	}
	if m["level"] != "info" {
		t.Errorf("expected level=info, got %v", m["level"])
	}
	if m["service.name"] != "blog-web" || m["service.version"] != "v1.4.2" {
		t.Errorf("unexpected service coordinates: %v", m)
	}
	if m["trace_id"] != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("expected trace_id from traceparent, got %v", m["trace_id"])
	}
	if m["http.request.method"] != "GET" || m["url.path"] != "/healthz" {
		t.Errorf("unexpected method/path: %v", m)
	}
	if m["http.response.status_code"] != float64(200) {
		t.Errorf("expected status 200, got %v", m["http.response.status_code"])
	}
	if dur, ok := m["http.server.request.duration"].(float64); !ok || dur < 0 {
		t.Errorf("expected non-negative duration float, got %v", m["http.server.request.duration"])
	}

	// 2. Request without traceparent generates 32-lowercase-hex trace_id
	buf.Reset()
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/posts/missing-slug", nil))
	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", rec.Code)
	}
	m = nil
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("expected JSON access log line, got %q (err=%v)", buf.String(), err)
	}
	tid, _ := m["trace_id"].(string)
	if !regexp.MustCompile(`^[0-9a-f]{32}$`).MatchString(tid) {
		t.Errorf("expected 32-hex generated trace_id, got %q", tid)
	}
	if m["http.response.status_code"] != float64(404) || m["level"] != "info" {
		t.Errorf("expected 404 at level=info, got status=%v level=%v", m["http.response.status_code"], m["level"])
	}

	// 3. 500 error logs at level=error with innermost unwrapped exception.type and exception.message
	buf.Reset()
	pathErr := &os.PathError{Op: "open", Path: "/var/run/blog-content", Err: os.ErrPermission}
	wrappedErr := fmt.Errorf("index failure: %w", pathErr)
	errHandler := server.NewHandler(&failingStore{err: wrappedErr}, nil, "", discardViews())
	rec = httptest.NewRecorder()
	errHandler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/posts", nil))
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("expected 500, got %d", rec.Code)
	}
	m = nil
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatalf("expected JSON access log line, got %q (err=%v)", buf.String(), err)
	}
	if m["level"] != "error" {
		t.Errorf("expected level=error for 500 response, got %v", m["level"])
	}
	if m["http.response.status_code"] != float64(500) {
		t.Errorf("expected http.response.status_code=500, got %v", m["http.response.status_code"])
	}
	wantType := fmt.Sprintf("%T", pathErr)
	if m["exception.type"] != wantType {
		t.Errorf("expected exception.type=%q, got %v", wantType, m["exception.type"])
	}
	if m["exception.message"] != wrappedErr.Error() {
		t.Errorf("expected exception.message=%q, got %v", wrappedErr.Error(), m["exception.message"])
	}
}

func TestRouter_RecordsEveryPageViewOnSuccessfulResponses(t *testing.T) {
	var views bytes.Buffer
	h := setupTestServerWithViews(t, analytics.New(logx.New(&views, "blog-web", "v1.4.2")))
	get := func(path, ua string) int {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("User-Agent", ua)
		req.Header.Set("X-Real-Ip", "198.51.100.23")
		req.Header.Set("traceparent", "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		return rec.Code
	}
	const browser = "Mozilla/5.0 (Macintosh; Intel Mac OS X 14_0) Safari/605.1.15"

	for _, tc := range []struct {
		path, ua string
		status   int
	}{
		{"/api/posts", browser, http.StatusOK},
		{"/api/posts/cilium-zero-plaintext-k3s", browser, http.StatusOK},
		{"/api/posts/cilium-zero-plaintext-k3s", browser, http.StatusOK},
		{"/api/posts/cilium-zero-plaintext-k3s", "Googlebot/2.1", http.StatusOK},
		{"/api/posts/no-such-post", browser, http.StatusNotFound},
		{"/api/posts/bad%21slug", browser, http.StatusBadRequest},
		{"/healthz", browser, http.StatusOK},
	} {
		if got := get(tc.path, tc.ua); got != tc.status {
			t.Fatalf("GET %s: status %d, want %d", tc.path, got, tc.status)
		}
	}

	var got []map[string]any
	for _, line := range strings.Split(strings.TrimSpace(views.String()), "\n") {
		var m map[string]any
		if err := json.Unmarshal([]byte(line), &m); err != nil {
			t.Fatalf("view line is not JSON: %q (%v)", line, err)
		}
		got = append(got, m)
	}
	if len(got) != 3 {
		t.Fatalf("want 3 view lines (home, post, post again), got %d:\n%s", len(got), views.String())
	}
	if got[0]["page.kind"] != "home" {
		t.Errorf("first view = %v, want home", got[0])
	}
	for _, m := range got[1:] {
		if m["page.kind"] != "post" || m["page.slug"] != "cilium-zero-plaintext-k3s" || m["page.topic"] != "cloud-native" {
			t.Errorf("view = %v, want post cilium-zero-plaintext-k3s/cloud-native", m)
		}
	}
	for _, m := range got {
		if m["event.name"] != "blog.analytics" {
			t.Errorf("event.name = %v", m["event.name"])
		}
		if m["visitor.id"] != got[0]["visitor.id"] {
			t.Errorf("visitor.id = %v, want %v for the same visitor", m["visitor.id"], got[0]["visitor.id"])
		}
		if _, ok := m["trace_id"]; ok {
			t.Errorf("analytics line must not carry trace_id: %v", m)
		}
	}
	if got[1]["event.id"] == got[2]["event.id"] {
		t.Errorf("repeat view reused event.id %v", got[1]["event.id"])
	}
}

func TestRouter_ServerErrorRecordsNoPageView(t *testing.T) {
	var views bytes.Buffer
	h := server.NewHandler(&failingStore{err: os.ErrPermission}, nil, "",
		analytics.New(logx.New(&views, "", "")))
	for _, path := range []string{"/api/posts", "/api/posts/some-post"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 Firefox/133.0")
		rec := httptest.NewRecorder()
		h.ServeHTTP(rec, req)
		if rec.Code != http.StatusInternalServerError {
			t.Fatalf("GET %s: status %d, want 500", path, rec.Code)
		}
	}
	if views.Len() != 0 {
		t.Fatalf("500 responses must not record views, got:\n%s", views.String())
	}
}
