package studio

import (
	"context"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

//go:embed all:ui/dist
var uiFS embed.FS

const maxJSONBody = 2 << 20

// Server serves the studio UI and /studio/api/* on a loopback address.
type Server struct {
	m     *Manager
	hub   *Hub
	hosts map[string]bool // allowed Host header values
	ui    fs.FS
	now   func() time.Time

	mu     sync.Mutex
	active string // draft whose worktree backs /content/* previews
}

// NewServer builds the handler. addr is the listen address (host:port); only
// requests addressed to it via a loopback name are accepted.
func NewServer(m *Manager, hub *Hub, addr string) (*Server, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if !isLoopbackHost(host) {
		return nil, fmt.Errorf("listen address %q refused: studio must bind a loopback address (127.0.0.1, localhost, or ::1)", addr)
	}
	ui, err := fs.Sub(uiFS, "ui/dist")
	if err != nil {
		return nil, err
	}
	return &Server{
		m: m, hub: hub, ui: ui, now: time.Now,
		hosts: map[string]bool{
			"127.0.0.1:" + port: true, "localhost:" + port: true, "[::1]:" + port: true,
		},
	}, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

// Handler returns the guarded HTTP handler.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /studio/api/state", s.state)
	mux.HandleFunc("POST /studio/api/drafts", s.createDraft)
	mux.HandleFunc("GET /studio/api/drafts/{key}", s.getDraft)
	mux.HandleFunc("PUT /studio/api/drafts/{key}", s.putDraft)
	mux.HandleFunc("DELETE /studio/api/drafts/{key}", s.discard)
	mux.HandleFunc("GET /studio/api/drafts/{key}/publish-plan", s.plan)
	mux.HandleFunc("POST /studio/api/drafts/{key}/{action}", s.postDraftAction)
	mux.HandleFunc("POST /studio/api/validate", s.validate)
	mux.HandleFunc("PUT /studio/api/active", s.setActive)
	mux.HandleFunc("GET /studio/api/events", s.events)
	mux.HandleFunc("GET /content/{kind}/{rel...}", s.contentFile)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/studio/api/") || r.Method != http.MethodGet && r.Method != http.MethodHead {
			writeErr(w, http.StatusNotFound, errors.New("not found"))
			return
		}
		s.spa(w, r)
	})
	return s.guard(mux)
}

func (s *Server) postDraftAction(w http.ResponseWriter, r *http.Request) {
	key, action := r.PathValue("key"), r.PathValue("action")
	if key == "from-post" {
		if !ValidKey(action) {
			writeErr(w, http.StatusBadRequest, fmt.Errorf("%w: %q", ErrInvalidKey, action))
			return
		}
		s.editPublishedSlug(w, r, action)
		return
	}
	switch action {
	case "validate":
		s.validate(w, r)
	case "save":
		s.save(w, r)
	case "publish":
		s.publish(w, r)
	case "assets":
		s.upload(w, r)
	default:
		writeErr(w, http.StatusNotFound, errors.New("not found"))
	}
}

// guard enforces loopback Host (DNS-rebinding) and same-origin mutations (CSRF).
func (s *Server) guard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.hosts[r.Host] {
			writeErr(w, http.StatusForbidden, fmt.Errorf("host %q not allowed", r.Host))
			return
		}
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			origin := r.Header.Get("Origin")
			if origin == "" || !strings.HasPrefix(origin, "http://") || !s.hosts[strings.TrimPrefix(origin, "http://")] {
				writeErr(w, http.StatusForbidden, errors.New("cross-origin request rejected"))
				return
			}
			ct := r.Header.Get("Content-Type")
			if r.Method != http.MethodDelete || r.ContentLength > 0 {
				if !strings.HasPrefix(ct, "application/json") && !strings.HasPrefix(ct, "multipart/form-data") {
					writeErr(w, http.StatusUnsupportedMediaType, errors.New("content type must be application/json"))
					return
				}
			}
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}

// ---------- helpers ----------

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func statusFor(err error) int {
	var ve *ValidationError
	switch {
	case errors.As(err, &ve):
		return http.StatusUnprocessableEntity
	case errors.Is(err, ErrDraftNotFound), errors.Is(err, ErrPostNotFound), errors.Is(err, fs.ErrNotExist):
		return http.StatusNotFound
	case errors.Is(err, ErrInvalidKey), errors.Is(err, ErrInvalidPath), errors.Is(err, ErrAssetType), errors.Is(err, ErrFrontmatter):
		return http.StatusBadRequest
	case errors.Is(err, ErrDraftExists), errors.Is(err, ErrConflict), errors.Is(err, ErrNothingToPublish),
		errors.Is(err, ErrNonContentPath), errors.Is(err, ErrPushRejected):
		return http.StatusConflict
	case errors.Is(err, ErrAssetTooLarge), errors.Is(err, ErrContentFull):
		return http.StatusRequestEntityTooLarge
	}
	return http.StatusInternalServerError
}

func fail(w http.ResponseWriter, err error) { writeErr(w, statusFor(err), err) }

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxJSONBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("invalid JSON: %w", err))
		return false
	}
	return true
}

func keyParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	key := r.PathValue("key")
	if !ValidKey(key) {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("%w: %q", ErrInvalidKey, key))
		return "", false
	}
	return key, true
}

// ---------- views ----------

// DraftView is a draft row for the sidebar.
type DraftView struct {
	DraftStatus
	Title string `json:"title"`
	Slug  string `json:"slug"`
	Date  string `json:"date"`
}

type stateResponse struct {
	PublishBranch string          `json:"publishBranch"`
	Remote        string          `json:"remote"`
	Repo          string          `json:"repo"`
	Active        string          `json:"active"`
	Drafts        []DraftView     `json:"drafts"`
	Published     []PublishedPost `json:"published"`
}

func (s *Server) state(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	drafts, err := s.m.List(ctx)
	if err != nil {
		fail(w, err)
		return
	}
	views := make([]DraftView, 0, len(drafts))
	for _, d := range drafts {
		v := DraftView{DraftStatus: d}
		if data, err := os.ReadFile(filepath.Join(d.Worktree, filepath.FromSlash(d.Path))); err == nil {
			if doc, err := ParseDocument(data); err == nil {
				v.Title, v.Slug, v.Date = doc.Frontmatter.Title, doc.Frontmatter.Slug, doc.Frontmatter.Date
			}
		}
		views = append(views, v)
	}
	published, err := s.m.PublishedPosts(ctx)
	if err != nil {
		fail(w, err)
		return
	}
	if published == nil {
		published = []PublishedPost{}
	}
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()
	writeJSON(w, http.StatusOK, stateResponse{
		PublishBranch: s.m.cfg.PublishBranch, Remote: s.m.cfg.Remote, Repo: s.m.Repo(),
		Active: active, Drafts: views, Published: published,
	})
}

type draftResponse struct {
	Draft    *DraftStatus `json:"draft"`
	Document *Document    `json:"document"`
	Issues   []Issue      `json:"issues"`
}

func (s *Server) draftPayload(ctx context.Context, key string) (*draftResponse, error) {
	st, err := s.m.Status(ctx, key)
	if err != nil {
		return nil, err
	}
	data, err := s.m.Read(ctx, key)
	if err != nil {
		return nil, err
	}
	doc, err := ParseDocument(data)
	if err != nil {
		// Hand-edited file with broken delimiters: show it all as body.
		doc = &Document{Frontmatter: Frontmatter{Tags: []string{}}, Body: string(data)}
	}
	issues, err := s.m.LintDraft(ctx, key, data)
	if err != nil {
		return nil, err
	}
	return &draftResponse{Draft: st, Document: doc, Issues: nonNil(issues)}, nil
}

func nonNil(i []Issue) []Issue {
	if i == nil {
		return []Issue{}
	}
	return i
}

// ---------- handlers ----------

func (s *Server) createDraft(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Title string `json:"title"`
	}
	if r.ContentLength != 0 && !decode(w, r, &req) {
		return
	}
	title := strings.TrimSpace(req.Title)
	if strings.ContainsAny(title, "\r\n") {
		writeErr(w, http.StatusBadRequest, errors.New("title must be a single line"))
		return
	}
	base := Slugify(title)
	if base == "" {
		base = "untitled"
	}
	ctx := r.Context()
	key := s.m.UniqueKey(ctx, base)
	date := s.now().Format("2006-01-02")
	data, err := SerializeDocument(&Document{
		Frontmatter: Frontmatter{Title: title, Slug: key, Date: date, Tags: []string{}},
		Body:        "Start writing here.\n\n## First section\n",
	})
	if err != nil {
		fail(w, err)
		return
	}
	d, err := s.m.Create(ctx, CreateOptions{Key: key, Path: PostPath(date, key), Content: data})
	if err != nil {
		fail(w, err)
		return
	}
	s.setActiveKey(d.Key)
	s.respondDraft(w, r, d.Key, http.StatusCreated)
}

func (s *Server) editPublishedSlug(w http.ResponseWriter, r *http.Request, slug string) {
	d, err := s.m.EditPublished(r.Context(), slug)
	if err != nil {
		fail(w, err)
		return
	}
	s.setActiveKey(d.Key)
	s.respondDraft(w, r, d.Key, http.StatusCreated)
}

func (s *Server) respondDraft(w http.ResponseWriter, r *http.Request, key string, status int) {
	p, err := s.draftPayload(r.Context(), key)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, status, p)
}

func (s *Server) getDraft(w http.ResponseWriter, r *http.Request) {
	if key, ok := keyParam(w, r); ok {
		s.respondDraft(w, r, key, http.StatusOK)
	}
}

func (s *Server) putDraft(w http.ResponseWriter, r *http.Request) {
	key, ok := keyParam(w, r)
	if !ok {
		return
	}
	var doc Document
	if !decode(w, r, &doc) {
		return
	}
	data, err := SerializeDocument(&doc)
	if err != nil {
		fail(w, err)
		return
	}
	if err := s.m.Write(r.Context(), key, data); err != nil {
		fail(w, err)
		return
	}
	s.respondDraft(w, r, key, http.StatusOK)
}

func (s *Server) validate(w http.ResponseWriter, r *http.Request) {
	key := r.PathValue("key")
	if key == "" {
		s.mu.Lock()
		key = s.active
		s.mu.Unlock()
	} else if !ValidKey(key) {
		writeErr(w, http.StatusBadRequest, fmt.Errorf("%w: %q", ErrInvalidKey, key))
		return
	}
	var doc Document
	if !decode(w, r, &doc) {
		return
	}
	data, err := SerializeDocument(&doc)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"issues": []Issue{{Severity: SevError, Message: err.Error()}}})
		return
	}
	var issues []Issue
	if key != "" {
		issues, err = s.m.LintDraft(r.Context(), key, data)
		if err != nil {
			fail(w, err)
			return
		}
	} else {
		issues = Lint(data, PostPath(doc.Frontmatter.Date, doc.Frontmatter.Slug), LintOptions{})
	}
	writeJSON(w, http.StatusOK, map[string]any{"issues": nonNil(issues)})
}

func (s *Server) save(w http.ResponseWriter, r *http.Request) {
	key, ok := keyParam(w, r)
	if !ok {
		return
	}
	res, err := s.m.Save(r.Context(), key)
	switch {
	case errors.Is(err, ErrNothingToCommit):
		writeJSON(w, http.StatusOK, map[string]any{"nothingToCommit": true})
	case err != nil:
		fail(w, err)
	default:
		writeJSON(w, http.StatusOK, res)
	}
}

func (s *Server) plan(w http.ResponseWriter, r *http.Request) {
	key, ok := keyParam(w, r)
	if !ok {
		return
	}
	p, err := s.m.Plan(r.Context(), key)
	if err != nil {
		fail(w, err)
		return
	}
	p.Issues = nonNil(p.Issues)
	if p.Files == nil {
		p.Files = []string{}
	}
	writeJSON(w, http.StatusOK, p)
}

// publish streams newline-delimited JSON: {"step":"fetch"} … then a final
// {"result":{…}} or {"error":"…","status":409}.
func (s *Server) publish(w http.ResponseWriter, r *http.Request) {
	key, ok := keyParam(w, r)
	if !ok {
		return
	}
	var req struct {
		Confirm string `json:"confirm"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Confirm != key {
		writeErr(w, http.StatusBadRequest, errors.New(`publish requires {"confirm": "<key>"}`))
		return
	}

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusOK)
	flusher, _ := w.(http.Flusher)
	enc := json.NewEncoder(w)
	emit := func(v any) {
		_ = enc.Encode(v)
		if flusher != nil {
			flusher.Flush()
		}
	}
	// Publishing must not be cut short by the tab closing mid-push.
	ctx := context.WithoutCancel(r.Context())
	res, err := s.m.Publish(ctx, key, func(step string) { emit(map[string]string{"step": step}) })
	if err != nil {
		emit(map[string]any{"error": err.Error(), "status": statusFor(err)})
		return
	}
	s.mu.Lock()
	if s.active == key {
		s.active = ""
	}
	s.mu.Unlock()
	emit(map[string]any{"result": res})
}

func (s *Server) discard(w http.ResponseWriter, r *http.Request) {
	key, ok := keyParam(w, r)
	if !ok {
		return
	}
	var req struct {
		Confirm string `json:"confirm"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Confirm != key {
		writeErr(w, http.StatusBadRequest, errors.New(`discard requires {"confirm": "<key>"}`))
		return
	}
	if err := s.m.Discard(r.Context(), key); err != nil {
		fail(w, err)
		return
	}
	s.mu.Lock()
	if s.active == key {
		s.active = ""
	}
	s.mu.Unlock()
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) upload(w http.ResponseWriter, r *http.Request) {
	key, ok := keyParam(w, r)
	if !ok {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, MaxAssetBytes+(1<<20))
	file, hdr, err := r.FormFile("file")
	if err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			fail(w, fmt.Errorf("%w: exceeds 10 MiB", ErrAssetTooLarge))
			return
		}
		writeErr(w, http.StatusBadRequest, fmt.Errorf("multipart field \"file\" required: %w", err))
		return
	}
	defer file.Close()
	res, err := s.m.AddAsset(r.Context(), key, hdr.Filename, file)
	if err != nil {
		fail(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, res)
}

func (s *Server) setActive(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Key != "" && !ValidKey(req.Key) {
		writeErr(w, http.StatusBadRequest, ErrInvalidKey)
		return
	}
	s.setActiveKey(req.Key)
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) setActiveKey(key string) {
	s.mu.Lock()
	s.active = key
	s.mu.Unlock()
}

func (s *Server) events(w http.ResponseWriter, r *http.Request) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeErr(w, http.StatusInternalServerError, errors.New("streaming unsupported"))
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-store")
	history, ch, cancel := s.hub.Subscribe()
	defer cancel()

	send := func(e Event) bool {
		b, _ := json.Marshal(e)
		if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
			return false
		}
		flusher.Flush()
		return true
	}
	for _, e := range history {
		if !send(e) {
			return
		}
	}
	flusher.Flush()
	keepalive := time.NewTicker(25 * time.Second)
	defer keepalive.Stop()
	for {
		select {
		case <-r.Context().Done():
			return
		case e, ok := <-ch:
			if !ok || !send(e) {
				return
			}
		case <-keepalive.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

// contentFile serves /content/assets/* and /content/demos/* for the preview,
// from the active draft's worktree, falling back to the main checkout.
// Headers mirror internal/server/router.go.
func (s *Server) contentFile(w http.ResponseWriter, r *http.Request) {
	kind, rel := r.PathValue("kind"), r.PathValue("rel")
	s.mu.Lock()
	active := s.active
	s.mu.Unlock()

	full := ""
	if active != "" {
		if p, err := s.m.ResolveContentFile(r.Context(), active, kind, rel); err == nil {
			full = p
		}
	}
	if full == "" {
		p, err := resolveUnder(filepath.Join(s.m.Repo(), "content", kind), kind, rel)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		full = p
	}
	w.Header().Set("Cache-Control", "no-store")
	if kind == "demos" {
		w.Header().Set("Content-Security-Policy", "default-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'none'; frame-ancestors 'self'")
	}
	http.ServeFile(w, r, full)
}

func resolveUnder(root, kind, rel string) (string, error) {
	if kind != "assets" && kind != "demos" {
		return "", fs.ErrNotExist
	}
	clean := filepath.Clean(filepath.FromSlash(rel))
	if clean == "." || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return "", fs.ErrNotExist
	}
	full := filepath.Join(root, clean)
	fi, err := os.Lstat(full)
	if err != nil || !fi.Mode().IsRegular() {
		return "", fs.ErrNotExist
	}
	return full, nil
}

const notBuiltPage = `<!doctype html><meta charset="utf-8"><title>Blog Studio</title>
<body style="font-family:system-ui;background:#fafaf9;color:#0c0a09;padding:48px">
<h1 style="font-family:Georgia,serif">Blog Studio UI is not built</h1>
<p>Run <code>make studio</code> (or <code>npm --prefix studio ci &amp;&amp; npm --prefix studio run build</code>) and rebuild the binary.</p>`

// spa serves the embedded UI with an index.html fallback for client routes.
func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
	if p == "" {
		p = "index.html"
	}
	if fi, err := fs.Stat(s.ui, p); err != nil || fi.IsDir() {
		p = "index.html"
	}
	if _, err := fs.Stat(s.ui, p); err != nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = io.WriteString(w, notBuiltPage)
		return
	}
	if p == "index.html" {
		w.Header().Set("Cache-Control", "no-store")
	}
	w.Header().Set("Content-Security-Policy", "default-src 'self'; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; frame-src 'self'; connect-src 'self'; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
	http.ServeFileFS(w, r, s.ui, p)
}
