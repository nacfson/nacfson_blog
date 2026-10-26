package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/nacfson/nacfson_blog/internal/content"
)

// NewHandler constructs and configures the HTTP router for the blog service.
func NewHandler(store content.ContentStore, activeContentDir func() string, webOutDir string) http.Handler {
	mux := http.NewServeMux()

	// GET /healthz -> 200 OK {"status":"ok"}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// GET /api/posts -> JSON list from store.ListPosts
	mux.HandleFunc("GET /api/posts", func(w http.ResponseWriter, r *http.Request) {
		topic := r.URL.Query().Get("topic")
		tag := r.URL.Query().Get("tag")
		posts, err := store.ListPosts(r.Context(), topic, tag)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if posts == nil {
			posts = []content.PostSummary{}
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(posts)
	})

	// GET /api/posts/{slug} -> JSON post from store.GetPost
	mux.HandleFunc("GET /api/posts/{slug}", func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		post, err := store.GetPost(r.Context(), slug)
		if err != nil {
			if errors.Is(err, content.ErrInvalidSlug) {
				http.Error(w, "invalid post slug", http.StatusBadRequest)
				return
			}
			if errors.Is(err, content.ErrPostNotFound) {
				http.Error(w, "post not found", http.StatusNotFound)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(post)
	})

	// GET /content/assets/{filepath...} -> validates clean relative path inside assets dir
	mux.HandleFunc("GET /content/assets/{filepath...}", func(w http.ResponseWriter, r *http.Request) {
		relPath := r.PathValue("filepath")
		cleanRel := filepath.Clean(relPath)
		if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		if activeContentDir == nil {
			http.NotFound(w, r)
			return
		}
		activeRoot := activeContentDir()
		if activeRoot == "" {
			http.NotFound(w, r)
			return
		}
		assetsDir := filepath.Join(activeRoot, "assets")
		fullPath := filepath.Join(assetsDir, cleanRel)
		rel, err := filepath.Rel(assetsDir, fullPath)
		if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		fi, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if fi.IsDir() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Cache-Control", "public, max-age=300")
		http.ServeFile(w, r, fullPath)
	})

	// GET /content/demos/{filepath...} -> validates clean relative path inside demos dir
	mux.HandleFunc("GET /content/demos/{filepath...}", func(w http.ResponseWriter, r *http.Request) {
		relPath := r.PathValue("filepath")
		cleanRel := filepath.Clean(relPath)
		if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		if activeContentDir == nil {
			http.NotFound(w, r)
			return
		}
		activeRoot := activeContentDir()
		if activeRoot == "" {
			http.NotFound(w, r)
			return
		}
		demosDir := filepath.Join(activeRoot, "demos")
		fullPath := filepath.Join(demosDir, cleanRel)
		rel, err := filepath.Rel(demosDir, fullPath)
		if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		fi, err := os.Stat(fullPath)
		if err != nil {
			if os.IsNotExist(err) {
				http.NotFound(w, r)
				return
			}
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if fi.IsDir() {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Security-Policy", "default-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'none'; frame-ancestors 'self'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		http.ServeFile(w, r, fullPath)
	})

	// GET /{path...} -> serves static files from webOutDir with SPA fallback
	mux.HandleFunc("GET /{path...}", func(w http.ResponseWriter, r *http.Request) {
		if webOutDir == "" {
			http.NotFound(w, r)
			return
		}
		reqPath := r.URL.Path
		if strings.HasPrefix(reqPath, "/api/") || reqPath == "/api" ||
			strings.HasPrefix(reqPath, "/content/") || reqPath == "/content" {
			http.NotFound(w, r)
			return
		}
		cleanRel := filepath.Clean(strings.TrimPrefix(reqPath, "/"))
		if strings.HasPrefix(cleanRel, "..") || filepath.IsAbs(cleanRel) {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		targetPath := filepath.Join(webOutDir, filepath.FromSlash(cleanRel))
		rel, err := filepath.Rel(webOutDir, targetPath)
		if err != nil || strings.HasPrefix(rel, "..") {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		// If a static file exists at targetPath and is not a directory, serve it directly
		if fi, err := os.Stat(targetPath); err == nil && !fi.IsDir() {
			http.ServeFile(w, r, targetPath)
			return
		}

		// SPA route fallback:
		// If request is for /post or /post/*, serve post/index.html if present
		if reqPath == "/post" || strings.HasPrefix(reqPath, "/post/") || cleanRel == "post" || strings.HasPrefix(cleanRel, "post/") {
			postIndex := filepath.Join(webOutDir, "post", "index.html")
			if fi, err := os.Stat(postIndex); err == nil && !fi.IsDir() {
				http.ServeFile(w, r, postIndex)
				return
			}
		}

		// Default SPA fallback: index.html
		rootIndex := filepath.Join(webOutDir, "index.html")
		if fi, err := os.Stat(rootIndex); err == nil && !fi.IsDir() {
			http.ServeFile(w, r, rootIndex)
			return
		}

		http.NotFound(w, r)
	})

	// Return handler wrapped with path traversal guard
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "..") || strings.Contains(r.URL.RawPath, "..") {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
