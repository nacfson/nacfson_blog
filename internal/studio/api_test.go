package studio

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"
)

type apiEnv struct {
	*env
	hub  *Hub
	srv  *Server
	ts   *httptest.Server
	addr string
}

func newAPIEnv(t *testing.T) *apiEnv {
	t.Helper()
	e := newEnv(t)
	hub := NewHub(32)
	prev := e.m.git.log
	e.m.git.log = func(ev Event) {
		if prev != nil {
			prev(ev)
		}
		hub.Publish(ev)
	}

	ts := httptest.NewUnstartedServer(nil)
	t.Cleanup(ts.Close)
	addr := ts.Listener.Addr().String()
	srv, err := NewServer(e.m, hub, addr)
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	srv.now = func() time.Time { return time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC) }
	ts.Config.Handler = srv.Handler()
	ts.Start()

	return &apiEnv{env: e, hub: hub, srv: srv, ts: ts, addr: addr}
}

func (a *apiEnv) do(t *testing.T, method, path string, body any) *http.Response {
	t.Helper()
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			t.Fatal(err)
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, "http://"+a.addr+path, r)
	if err != nil {
		t.Fatal(err)
	}
	if method != http.MethodGet && method != http.MethodHead {
		req.Header.Set("Origin", "http://"+a.addr)
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	return resp
}

func readJSON[T any](t *testing.T, resp *http.Response) T {
	t.Helper()
	defer resp.Body.Close()
	var v T
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatalf("decode JSON (status %d): %v", resp.StatusCode, err)
	}
	return v
}

func TestNewServer_RefusesNonLoopback(t *testing.T) {
	e := newEnv(t)
	hub := NewHub(8)
	for _, bad := range []string{"0.0.0.0:4321", ":4321", "192.168.1.10:4321", "example.com:4321", "invalid"} {
		if _, err := NewServer(e.m, hub, bad); err == nil {
			t.Errorf("NewServer(%q) succeeded; want error", bad)
		}
	}
	for _, ok := range []string{"127.0.0.1:4321", "localhost:4321", "[::1]:4321"} {
		if _, err := NewServer(e.m, hub, ok); err != nil {
			t.Errorf("NewServer(%q): %v", ok, err)
		}
	}
}

func TestAPI_Guards(t *testing.T) {
	a := newAPIEnv(t)

	// DNS rebinding: wrong Host header.
	req, _ := http.NewRequest(http.MethodGet, "http://"+a.addr+"/studio/api/state", nil)
	req.Host = "evil.example:80"
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("wrong Host: status = %d, want 403", resp.StatusCode)
	}

	// CSRF: missing or cross-origin Origin on POST.
	for _, origin := range []string{"", "http://evil.example", "https://" + a.addr, "http://127.0.0.1:9999"} {
		req, _ := http.NewRequest(http.MethodPost, "http://"+a.addr+"/studio/api/drafts", strings.NewReader(`{}`))
		if origin != "" {
			req.Header.Set("Origin", origin)
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusForbidden {
			t.Errorf("Origin %q: status = %d, want 403", origin, resp.StatusCode)
		}
	}

	// Simple-request CSRF: text/plain Content-Type rejected.
	req, _ = http.NewRequest(http.MethodPost, "http://"+a.addr+"/studio/api/drafts", strings.NewReader(`{}`))
	req.Header.Set("Origin", "http://"+a.addr)
	req.Header.Set("Content-Type", "text/plain")
	resp, err = http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnsupportedMediaType {
		t.Errorf("text/plain POST: status = %d, want 415", resp.StatusCode)
	}

	// Invalid slug key in URL path.
	for _, path := range []string{
		"/studio/api/drafts/bad..slug",
		"/studio/api/drafts/publish",
		"/studio/api/drafts/-leading-dash",
	} {
		resp := a.do(t, http.MethodGet, path, nil)
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Errorf("GET %s: status = %d, want 400", path, resp.StatusCode)
		}
	}

	// Unknown API route returns JSON 404, not SPA fallback.
	resp = a.do(t, http.MethodGet, "/studio/api/nope", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("unknown API route: status = %d, want 404", resp.StatusCode)
	}
}

func TestAPI_EndToEndDraftSavePublishFlow(t *testing.T) {
	a := newAPIEnv(t)

	// 1. Initial state has 1 published post ("existing") and 0 drafts.
	resp := a.do(t, http.MethodGet, "/studio/api/state", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("state: %d", resp.StatusCode)
	}
	st := readJSON[stateResponse](t, resp)
	if st.PublishBranch != "main" || len(st.Drafts) != 0 || len(st.Published) != 1 || st.Published[0].Summary.Slug != "existing" {
		t.Fatalf("initial state = %+v", st)
	}

	// 2. Create draft.
	resp = a.do(t, http.MethodPost, "/studio/api/drafts", map[string]string{"title": "Hello Studio"})
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("createDraft: %d", resp.StatusCode)
	}
	created := readJSON[draftResponse](t, resp)
	if created.Draft.Key != "hello-studio" || created.Draft.State != StateNew {
		t.Fatalf("created draft = %+v", created.Draft)
	}
	// Missing topic produces a lint error initially.
	if len(created.Issues) == 0 {
		t.Fatal("expected initial lint issues (missing topic)")
	}

	// 3. Autosave (PUT) with valid frontmatter and body.
	doc := Document{
		Frontmatter: Frontmatter{
			Title:   "Hello Studio",
			Slug:    "hello-studio",
			Date:    "2026-11-04",
			Topic:   "ai-agents",
			Summary: "Testing the studio HTTP API.",
			Tags:    []string{"go", "studio"},
		},
		Body: "## Intro\n\nBody paragraph with *italics* warning.\n",
	}
	resp = a.do(t, http.MethodPut, "/studio/api/drafts/hello-studio", doc)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("putDraft: %d", resp.StatusCode)
	}
	updated := readJSON[draftResponse](t, resp)
	if updated.Draft.State != StateUnsaved {
		t.Errorf("state after PUT = %s, want unsaved", updated.Draft.State)
	}
	for _, iss := range updated.Issues {
		if iss.Severity == SevError {
			t.Errorf("unexpected error issue: %+v", iss)
		}
	}

	// 4. Standalone validate endpoint (both /studio/api/validate and /studio/api/drafts/{key}/validate).
	for _, vpath := range []string{"/studio/api/validate", "/studio/api/drafts/hello-studio/validate"} {
		resp = a.do(t, http.MethodPost, vpath, doc)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: %d", vpath, resp.StatusCode)
		}
		vres := readJSON[struct{ Issues []Issue }](t, resp)
		if len(vres.Issues) != 1 || vres.Issues[0].Severity != SevWarn {
			t.Errorf("%s issues = %+v, want 1 warning", vpath, vres.Issues)
		}
	}

	// 5. Save commits to the draft branch.
	resp = a.do(t, http.MethodPost, "/studio/api/drafts/hello-studio/save", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("save: %d", resp.StatusCode)
	}
	sres := readJSON[SaveResult](t, resp)
	if sres.Commit == "" || sres.Path != "content/posts/2026-11-04-hello-studio.md" {
		t.Errorf("save result = %+v", sres)
	}
	// Second save with no changes returns nothingToCommit.
	resp = a.do(t, http.MethodPost, "/studio/api/drafts/hello-studio/save", nil)
	ntc := readJSON[map[string]bool](t, resp)
	if !ntc["nothingToCommit"] {
		t.Errorf("second save = %+v, want nothingToCommit", ntc)
	}

	// 6. Publish plan shows diff and no blocking errors.
	resp = a.do(t, http.MethodGet, "/studio/api/drafts/hello-studio/publish-plan", nil)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("plan: %d", resp.StatusCode)
	}
	plan := readJSON[PublishPlan](t, resp)
	if plan.Blocking || !strings.Contains(plan.Diff, "Hello Studio") || len(plan.Files) != 1 {
		t.Errorf("plan = %+v", plan)
	}

	// 7. Publish requires confirmation matching the key.
	resp = a.do(t, http.MethodPost, "/studio/api/drafts/hello-studio/publish", map[string]string{"confirm": "wrong"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("publish wrong confirm: %d, want 400", resp.StatusCode)
	}

	// 8. Real publish streams NDJSON steps + final result.
	resp = a.do(t, http.MethodPost, "/studio/api/drafts/hello-studio/publish", map[string]string{"confirm": "hello-studio"})
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("publish: %d", resp.StatusCode)
	}
	var steps []string
	var final PublishResult
	sc := bufio.NewScanner(resp.Body)
	for sc.Scan() {
		var msg struct {
			Step   string         `json:"step"`
			Result *PublishResult `json:"result"`
			Error  string         `json:"error"`
		}
		if err := json.Unmarshal(sc.Bytes(), &msg); err != nil {
			t.Fatalf("ndjson line %q: %v", sc.Text(), err)
		}
		if msg.Error != "" {
			t.Fatalf("publish stream error: %s", msg.Error)
		}
		if msg.Step != "" {
			steps = append(steps, msg.Step)
		}
		if msg.Result != nil {
			final = *msg.Result
		}
	}
	resp.Body.Close()
	if final.Slug != "hello-studio" || len(final.Commit) != 40 || len(steps) == 0 {
		t.Errorf("publish final = %+v, steps = %v", final, steps)
	}

	// 9. State now lists 0 drafts and 2 published posts.
	resp = a.do(t, http.MethodGet, "/studio/api/state", nil)
	st = readJSON[stateResponse](t, resp)
	if len(st.Drafts) != 0 || len(st.Published) != 2 {
		t.Errorf("state after publish: drafts=%d published=%d", len(st.Drafts), len(st.Published))
	}
}

func TestAPI_EditPublishedDiscardAndAssets(t *testing.T) {
	a := newAPIEnv(t)

	// Start editing the published post "existing".
	resp := a.do(t, http.MethodPost, "/studio/api/drafts/from-post/existing", nil)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("from-post: %d", resp.StatusCode)
	}
	dr := readJSON[draftResponse](t, resp)
	if dr.Draft.Key != "existing" || dr.Document.Frontmatter.Title != "Existing" {
		t.Fatalf("from-post response = %+v", dr)
	}

	// Upload an SVG asset and a demo HTML via multipart.
	upload := func(name, content string) (*http.Response, AssetResult) {
		var buf bytes.Buffer
		mw := multipart.NewWriter(&buf)
		fw, err := mw.CreateFormFile("file", name)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = io.WriteString(fw, content)
		_ = mw.Close()

		req, _ := http.NewRequest(http.MethodPost, "http://"+a.addr+"/studio/api/drafts/existing/assets", &buf)
		req.Header.Set("Origin", "http://"+a.addr)
		req.Header.Set("Content-Type", mw.FormDataContentType())
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		if res.StatusCode != http.StatusCreated {
			return res, AssetResult{}
		}
		return res, readJSON[AssetResult](t, res)
	}

	_, svg := upload("Chart Diagram.svg", `<svg xmlns="http://www.w3.org/2000/svg"></svg>`)
	if svg.URL != "/content/assets/chart-diagram.svg" {
		t.Errorf("svg URL = %q", svg.URL)
	}
	_, demo := upload("demo.html", `<!doctype html><script>fetch('/x')</script>`)
	if demo.URL != "/content/demos/demo.html" || len(demo.Warnings) == 0 {
		t.Errorf("demo = %+v", demo)
	}

	// Active draft serves uploaded content with production CSP on demos.
	resp = a.do(t, http.MethodGet, demo.URL, nil)
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK || !strings.Contains(string(body), "fetch") {
		t.Errorf("GET %s: status=%d body=%q", demo.URL, resp.StatusCode, body)
	}
	if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "connect-src 'none'") {
		t.Errorf("demo CSP = %q", csp)
	}

	// Discard requires confirmation, then deletes the draft.
	resp = a.do(t, http.MethodDelete, "/studio/api/drafts/existing", map[string]string{"confirm": "nope"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("discard wrong confirm: %d, want 400", resp.StatusCode)
	}
	resp = a.do(t, http.MethodDelete, "/studio/api/drafts/existing", map[string]string{"confirm": "existing"})
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("discard: %d, want 204", resp.StatusCode)
	}
}

func TestAPI_SSEEventsAndSPA(t *testing.T) {
	a := newAPIEnv(t)

	// Trigger a git event so history is non-empty.
	resp := a.do(t, http.MethodPost, "/studio/api/drafts", map[string]string{"title": "SSE Test"})
	resp.Body.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "http://"+a.addr+"/studio/api/events", nil)
	sresp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer sresp.Body.Close()
	sc := bufio.NewScanner(sresp.Body)
	if !sc.Scan() || !strings.HasPrefix(sc.Text(), "data: ") {
		t.Fatalf("expected SSE data line, got %q (err %v)", sc.Text(), sc.Err())
	}

	// When the SPA is not built (no index.html), GET / returns 503 with instructions.
	a.srv.ui = fstest.MapFS{}
	resp = a.do(t, http.MethodGet, "/", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("unbuilt SPA status = %d, want 503", resp.StatusCode)
	}

	// With an index.html present, GET / and client routes serve it with CSP.
	a.srv.ui = fstest.MapFS{
		"index.html":    &fstest.MapFile{Data: []byte("<!doctype html><title>Studio</title>")},
		"assets/app.js": &fstest.MapFile{Data: []byte("console.log(1)")},
	}
	for _, p := range []string{"/", "/drafts/foo"} {
		resp = a.do(t, http.MethodGet, p, nil)
		b, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK || !strings.Contains(string(b), "<title>Studio</title>") {
			t.Errorf("GET %s: status=%d body=%q", p, resp.StatusCode, b)
		}
		if csp := resp.Header.Get("Content-Security-Policy"); !strings.Contains(csp, "frame-ancestors 'none'") {
			t.Errorf("SPA CSP = %q", csp)
		}
	}

	// Fallback content file from main checkout when active draft doesn't have it.
	if err := os.MkdirAll(filepath.Join(a.repo, "content", "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(a.repo, "content", "assets", "main.svg"), []byte("<svg/>"), 0o644); err != nil {
		t.Fatal(err)
	}
	resp = a.do(t, http.MethodGet, "/content/assets/main.svg", nil)
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("fallback asset status = %d, want 200", resp.StatusCode)
	}
}
