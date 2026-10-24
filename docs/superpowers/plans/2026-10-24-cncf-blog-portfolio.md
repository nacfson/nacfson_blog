# CNCF Blog & Portfolio (`proj-blog`) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `nacfson_blog` (`proj-blog` at `https://blog.nacfson.cloud`), a single-pod Light-Theme Kimi-style interactive engineering blog and portfolio backed by a Go 1.22 stdlib server, Folder Mode `ContentStore`, and zero-image-rebuild Flux `source-controller` live content sync inside `nacfson_pipeline`.

**Architecture:** A single container (`blog-web`, running as `UID 10001` with `readOnlyRootFilesystem: true`) embeds a Go 1.22 stdlib HTTP server (`cmd/blog-server`) and a compiled Next.js static export (`web/out`). Code changes (`web/**`, `cmd/**`, `internal/**`) trigger a multi-arch GHCR Docker build (`Track A`), whereas content changes (`content/**`) skip the Docker build and are synced live every 30s from FluxCD's existing `source-controller` (`GitRepository/nacfson-blog-content` with `ignore: /* !/content/`) into an in-pod `emptyDir` volume (`/var/run/blog-content`) (`Track B`).

**Tech Stack:** Go 1.22 (`net/http`, `archive/tar`, `compress/gzip`, stdlib routing), Node.js 22 LTS (`node:22-alpine` build stage), TypeScript 5.5 (`ES2022` target), Next.js 14.2 (App Router `output: 'export'`), React 18.3, Tailwind CSS 3.4 (Light Theme), Vitest 2.0 + React Testing Library 16 (`jsdom`), FluxCD v2 (`source.toolkit.fluxcd.io/v1`, `kustomize.toolkit.fluxcd.io/v1`), Kubernetes (`restricted` PSS).

**Spec:** `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/docs/superpowers/specs/2026-10-24-cncf-blog-portfolio-design.md`

## Global Constraints

- **Go Version & Dependencies**: Go 1.22 stdlib (`net/http`, `archive/tar`, `compress/gzip`, `encoding/json`, `os`, `path/filepath`); zero external HTTP router or web framework dependencies.
- **JavaScript / TypeScript / Frontend Versions**:
  - **Build Runtime**: Node.js `>=22.0.0` (`node:22-alpine` in `Dockerfile` Stage 1 and `node-version: '22'` in GitHub Actions).
  - **Language & Compiler Target**: TypeScript `~5.5.4` compiling to ECMAScript `ES2022` (`target: "ES2022"`, `lib: ["dom", "dom.iterable", "es2022"]`, `module: "esnext"`, `moduleResolution: "bundler"`, `strict: true`).
  - **UI Framework**: Next.js `14.2.15` (App Router with `output: 'export'` and `images: { unoptimized: true }`), React `18.3.1`, React DOM `18.3.1`.
  - **Styling & Testing**: Tailwind CSS `3.4.13`, PostCSS `8.4.47`, Autoprefixer `10.4.20`, Vitest `2.1.3`, `@vitejs/plugin-react` `4.3.2`, `@testing-library/react` `16.0.1`, `@testing-library/jest-dom` `6.5.0`, `jsdom` `25.0.1`.
- **Security Context (`CONTRACT_ISOLATION_BUDGET.md`)**: `runAsNonRoot: true`, `runAsUser: 10001`, `runAsGroup: 10001`, `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`, `seccompProfile.type: RuntimeDefault`, `automountServiceAccountToken: false`.
- **Strict QoS (`CONTRACT_ISOLATION_BUDGET.md`)**: `requests.cpu == limits.cpu` (`100m`) and `requests.memory == limits.memory` (`128Mi`) on `Deployment/blog-web`.
- **Auth Door (`CONTRACT_AUTH_DOOR.md`)**: `signIn: "none"`, `IngressRoute` on `entryPoints: ["websecure"]` with `tls: {}`, `Host(\`blog.nacfson.cloud\`)`, middleware `platform/strip-forged-headers`, and no `forward-auth` or `identity` egress.
- **Theme**: Strictly Light Theme (`#f8fafc` background canvas, `#ffffff` cards, `#0f172a` text, `#2563eb` accent).
- **Dual-Track Delivery**: `.github/workflows/publish-ghcr-image.yml` triggers only on `web/**`, `cmd/**`, `internal/**`, `go.mod`, `go.sum`, `Dockerfile`, and `.github/workflows/publish-ghcr-image.yml` (never on `content/**`).

## Review Focus

1. **Path Traversal / Zip-Slip in Tarball & Static Routes**: A crafted tar entry (`../../etc/passwd` or symlink type `tar.TypeSymlink`) or URL request (`/content/assets/../../etc/passwd`, `/api/posts/..%2fsecret`) must be rejected before touching the filesystem (`TestSyncOnce_RejectsZipSlipAndSymlinks` in Task 2; `TestRouter_RejectsPathTraversal` in Task 3).
2. **Corrupt or Unreachable Flux Tarball During Sync**: If `source-controller` returns HTTP 500 or a truncated/corrupt `.tar.gz`, `FluxTarballSyncer` must preserve the existing active content directory and return an error without wiping live posts (`TestSyncOnce_PreservesActiveContentOnCorruptTarball` in Task 2).
3. **Malformed JSON Inside Custom Markdown Fenced Blocks**: A syntax typo inside a ```` ```blogIframeTabs ```` or ```` ```benchmarkChart ```` JSON block in a `.md` file must render a non-crashing inline error banner for that block rather than throwing an unhandled React exception that blanks the entire article (`renders inline fallback banner when custom block JSON is malformed` in Task 4).
4. **Markdown Frontmatter Missing Required Fields or Invalid Pillar**: A `.md` file in `content/posts/` with missing `slug`/`title`/`date` or an unknown `pillar` value must be rejected by `FolderStore.Reload()` with a descriptive error rather than serving half-empty metadata (`TestFolderStore_RejectsInvalidFrontmatter` in Task 1).
5. **Interactive Demo Sandbox Escape (`CSP` & `iframe sandbox`)**: `/content/demos/*` responses must always include `Content-Security-Policy: default-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'none'; frame-ancestors 'self'` and `X-Content-Type-Options: nosniff` so interactive HTML demos cannot make external network calls or frame-bust (`TestRouter_DemoSandboxHeaders` in Task 3).

---

## File Structure

### Repository 1: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog`
- `go.mod` — Go 1.22 module definition (`github.com/nacfson/nacfson_blog`).
- `content/competencies/matrix.json` — Seed competency matrix across CNCF/Kubernetes, Security, and Systems engineering.
- `content/posts/2026-10-24-cilium-zero-plaintext-k3s.md` — Initial production post (promoted from `samples/`).
- `content/assets/*.svg` — Initial SVG hero layers and architecture diagrams (promoted from `samples/assets/`).
- `content/demos/hubble-flow-sim.html` — Initial interactive HTML simulator (promoted from `samples/demos/`).
- `internal/content/store.go` — `Pillar` enum, `PostSummary`, `Post`, and `ContentStore` interface.
- `internal/content/folder_store.go` — Thread-safe (`sync.RWMutex`) filesystem implementation of `ContentStore` reading from an active directory pointer.
- `internal/content/folder_store_test.go` — Unit tests for `FolderStore`.
- `internal/syncer/flux_tarball.go` — `FluxTarballSyncer` seeding `/var/run/blog-content` and polling Flux `source-controller` via `ETag` with atomic staging extraction.
- `internal/syncer/flux_tarball_test.go` — Unit tests for `FluxTarballSyncer` (304 handling, atomic swap, zip-slip/symlink rejection, corrupt archive preservation).
- `internal/server/router.go` — Go 1.22 `net/http` handler mounting `/healthz`, `/api/posts`, `/api/posts/{slug}`, `/api/competencies`, `/content/assets/*`, `/content/demos/*`, and SPA static fallback.
- `internal/server/router_test.go` — HTTP tests for API endpoints, CSP headers, and path traversal rejection.
- `cmd/blog-server/main.go` — Binary entrypoint wiring env vars (`PORT`, `CONTENT_ACTIVE_DIR`, `SEED_CONTENT_DIR`, `WEB_OUT_DIR`, `FLUX_ARTIFACT_URL`, `FLUX_POLL_INTERVAL`), `FolderStore`, `FluxTarballSyncer`, and `http.Server`.
- `web/package.json`, `web/next.config.mjs`, `web/tsconfig.json`, `web/tailwind.config.ts`, `web/postcss.config.mjs`, `web/vitest.config.ts` — Next.js static export + Tailwind + Vitest config.
- `web/src/lib/types.ts` — TypeScript interfaces matching Go `PostSummary`, `Post`, `CompetencyMatrix`, and custom block JSON schemas.
- `web/src/components/BlogV2AnimatedHero.tsx` — Layered scroll/drag parallax hero (`scrollHero`) with `IntersectionObserver` pause.
- `web/src/components/BlogV2MiniNav.tsx` — Sticky scroll-spy TOC tracking article `<h2>` sections.
- `web/src/components/blocks/BlogPicture.tsx` — Responsive figure renderer (`blogPicture`).
- `web/src/components/blocks/BlogIframeTabs.tsx` — Tabbed interactive HTML sandbox (`blogIframeTabs`) with `click-or-center` poster mounting and fullscreen toggle.
- `web/src/components/blocks/BenchmarkChart.tsx` — Light-theme grouped SVG bar chart (`benchmarkChart`).
- `web/src/components/blocks/BenchmarkTable.tsx` — Metric comparison table (`benchmarkTable`) highlighting `**bold**` best values.
- `web/src/components/MarkdownDispatcher.tsx` — Tokenizer/renderer dispatching standard Markdown and the 5 custom fenced JSON block types with error boundary fallback.
- `web/src/components/__tests__/MarkdownDispatcher.test.tsx` — Component tests for all 5 custom blocks, malformed JSON resilience, and `click-or-center` iframe mounting.
- `web/src/app/layout.tsx`, `web/src/app/globals.css`, `web/src/app/page.tsx`, `web/src/app/post/page.tsx` — Light-theme editorial shell, portfolio home (pillar filter + competency matrix + post feed), and article reader view.
- `Dockerfile` — 3-stage container build (`node:22-alpine` -> `golang:1.22-alpine` -> `alpine:3.20` non-root `10001:10001`).
- `.github/workflows/publish-ghcr-image.yml` — Track A CI workflow building and pushing `ghcr.io/nacfson/blog-web` only on code/build path changes.

### Repository 2: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog`
- `clusters/local-k3s/flux-system/gotk-sync.yaml` & `clusters/vps-k3s/flux-system/gotk-sync.yaml` — Add `GitRepository/nacfson-blog-content` (`ignore: /* !/content/`).
- `deploy/projects/blog/boundary/{kustomization.yaml,namespace.yaml,sign-in.yaml,service-account.yaml,reconciler-rbac.yaml,resource-quota.yaml,network-policy.yaml}` — Tenant boundary for `proj-blog`.
- `deploy/projects/blog/workloads/{kustomization.yaml,deployment.yaml,services.yaml,ingress-route.yaml}` — Single-pod `blog-web` workload manifests.
- `deploy/platform/governance/{manual-change-policy.yaml,base-network-policy.yaml}` — Allowlist `proj-blog` and `system:serviceaccount:proj-blog:blog-reconciler`, and allow `proj-blog` ingress to `flux-system/source-controller:9090`.
- `deploy/platform/tls/{certificates.yaml,tls-store.yaml}` — Add `Certificate/blog-tls` (`blog.nacfson.cloud`).
- `clusters/{local-k3s,vps-k3s}/layers.yaml` — Add `Kustomization/project-boundaries-blog` and `blog-tls` patch/dependency.
- `clusters/{local-k3s,vps-k3s}/projects/{proj-blog.yaml,kustomization.yaml}` — Add `Kustomization/proj-blog`.
- `scripts/render-invariants.py` — Add `("Namespace", "proj-blog", "kustomize.toolkit.fluxcd.io/prune", "disabled")` to `required_annotations`.

---

### Task 1: Go `ContentStore`, `FolderStore` & Seed Content (`internal/content/`)

**Files:**
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/go.mod`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/content/competencies/matrix.json`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/content/posts/2026-10-24-cilium-zero-plaintext-k3s.md` (copied from `samples/`, updating `/files/` paths to `/content/assets/` and `/content/demos/`)
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/content/assets/` (copied from `samples/assets/*.svg`)
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/content/demos/hubble-flow-sim.html` (copied from `samples/demos/hubble-flow-sim.html`)
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/internal/content/store.go`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/internal/content/folder_store.go`
- Test: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/internal/content/folder_store_test.go`

**Interfaces:**
- Consumes: none (foundational package)
- Produces (`package content`):
  - `type Pillar string` (`PillarProjectUpdates = "project-updates"`, `PillarCompetencyGrowth = "competency-growth"`, `PillarTechOpinions = "tech-opinions"`)
  - `type PostSummary struct { Slug, Title, Date string; Pillar Pillar; Project, Summary string; Tags []string; ReadTimeMinutes int }`
  - `type Post struct { PostSummary; MarkdownBody string }`
  - `type ContentStore interface { ListPosts(ctx context.Context, pillar, tag string) ([]PostSummary, error); GetPost(ctx context.Context, slug string) (*Post, error); GetCompetencyMatrix(ctx context.Context) ([]byte, error) }`
  - `func NewFolderStore(activeRoot func() string) (*FolderStore, error)`
  - `func (s *FolderStore) Reload() error`
  - `var ErrPostNotFound = errors.New("post not found")`
  - `var ErrInvalidSlug = errors.New("invalid post slug")`

- [ ] **Step 1: Initialize `go.mod`, seed `content/` from `samples/`, and write failing `FolderStore` unit tests**

Create `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/go.mod` (`module github.com/nacfson/nacfson_blog`, `go 1.22`), copy `samples/assets/*.svg` to `content/assets/`, copy `samples/demos/hubble-flow-sim.html` to `content/demos/`, copy `samples/2026-10-24-cilium-zero-plaintext-k3s.md` to `content/posts/2026-10-24-cilium-zero-plaintext-k3s.md` (replacing `/files/hubble-flow-sim.html` with `/content/demos/hubble-flow-sim.html` and `/files/` SVG URLs with `/content/assets/`), and create `content/competencies/matrix.json`. Then write `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/internal/content/folder_store_test.go`:

```go
package content_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/content"
)

func TestFolderStore_LoadsAndFiltersPosts(t *testing.T) {
	root, err := filepath.Abs("../../content")
	if err != nil {
		t.Fatalf("abs path: %v", err)
	}
	store, err := content.NewFolderStore(func() string { return root })
	if err != nil {
		t.Fatalf("NewFolderStore failed: %v", err)
	}

	posts, err := store.ListPosts(context.Background(), "", "")
	if err != nil || len(posts) == 0 {
		t.Fatalf("expected at least 1 seeded post, got %d (err=%v)", len(posts), err)
	}
	if posts[0].Slug != "cilium-zero-plaintext-k3s" {
		t.Errorf("expected slug cilium-zero-plaintext-k3s, got %q", posts[0].Slug)
	}

	filtered, err := store.ListPosts(context.Background(), "project-updates", "Cilium")
	if err != nil || len(filtered) != 1 {
		t.Fatalf("expected 1 post matching pillar+tag, got %d (err=%v)", len(filtered), err)
	}

	post, err := store.GetPost(context.Background(), "cilium-zero-plaintext-k3s")
	if err != nil {
		t.Fatalf("GetPost failed: %v", err)
	}
	if post.MarkdownBody == "" {
		t.Errorf("expected non-empty MarkdownBody")
	}

	matrix, err := store.GetCompetencyMatrix(context.Background())
	if err != nil || len(matrix) == 0 {
		t.Fatalf("expected non-empty competency matrix, err=%v", err)
	}
}

func TestFolderStore_RejectsInvalidSlugsAndMissingPosts(t *testing.T) {
	root, _ := filepath.Abs("../../content")
	store, err := content.NewFolderStore(func() string { return root })
	if err != nil {
		t.Fatalf("NewFolderStore: %v", err)
	}
	for _, bad := range []string{"../secret", "a/b", `a\b`, "bad\x00slug", ""} {
		if _, err := store.GetPost(context.Background(), bad); !errors.Is(err, content.ErrInvalidSlug) {
			t.Errorf("slug %q: expected ErrInvalidSlug, got %v", bad, err)
		}
	}
	if _, err := store.GetPost(context.Background(), "non-existent-post"); !errors.Is(err, content.ErrPostNotFound) {
		t.Errorf("expected ErrPostNotFound, got %v", err)
	}
}

func TestFolderStore_RejectsInvalidFrontmatter(t *testing.T) {
	tmp := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmp, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	badMD := "---\ntitle: \"Missing Slug and Bad Pillar\"\ndate: \"2026-10-24\"\npillar: \"invalid-pillar\"\n---\nBody\n"
	if err := os.WriteFile(filepath.Join(tmp, "posts", "bad.md"), []byte(badMD), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := content.NewFolderStore(func() string { return tmp }); err == nil {
		t.Fatalf("expected NewFolderStore to fail on invalid frontmatter")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/content/...`
Expected: FAIL (`cannot find package` / `undefined: content.NewFolderStore`)

- [ ] **Step 3: Implement `internal/content/store.go` and `internal/content/folder_store.go`**

Implement `store.go` with the exact types in `Interfaces`, and `folder_store.go` with a custom stdlib YAML frontmatter parser (splitting on leading `---\n` and `\n---\n`, parsing scalar string/int keys and bracketed `tags: ["a", "b"]` slices, validating required `title`, `slug`, `date`, and valid `Pillar` enum values `project-updates` | `competency-growth` | `tech-opinions`), sorting posts descending by `Date`, and protecting reads with `sync.RWMutex`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./internal/content/...`
Expected: PASS (`TestFolderStore_LoadsAndFiltersPosts`, `TestFolderStore_RejectsInvalidSlugsAndMissingPosts`, `TestFolderStore_RejectsInvalidFrontmatter`)

- [ ] **Step 5: Commit**

```bash
git add go.mod content/ internal/content/
git commit -m "feat(content): add FolderStore ContentStore implementation and seed content"
```

---

### Task 2: Flux Tarball Live Syncer (`internal/syncer/flux_tarball.go`)

**Files:**
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/internal/syncer/flux_tarball.go`
- Test: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/internal/syncer/flux_tarball_test.go`

**Interfaces:**
- Consumes: `Reload() error` (satisfied by `*content.FolderStore`)
- Produces (`package syncer`):
  - `type Reloader interface { Reload() error }`
  - `type FluxTarballSyncer struct`
  - `func NewFluxTarballSyncer(artifactURL, workspaceDir string, reloader Reloader) *FluxTarballSyncer`
  - `func (s *FluxTarballSyncer) ActiveContentDir() string`
  - `func (s *FluxTarballSyncer) SeedIfEmpty(seedDir string) error`
  - `func (s *FluxTarballSyncer) SyncOnce(ctx context.Context) (updated bool, err error)`
  - `func (s *FluxTarballSyncer) Start(ctx context.Context, interval time.Duration)`

- [ ] **Step 1: Write failing unit tests in `internal/syncer/flux_tarball_test.go`**

```go
package syncer_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/nacfson/nacfson_blog/internal/syncer"
)

type mockReloader struct{ count atomic.Int32 }

func (m *mockReloader) Reload() error {
	m.count.Add(1)
	return nil
}

func buildTarGz(t *testing.T, entries []tar.Header, bodies [][]byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)
	for i, hdr := range entries {
		if err := tw.WriteHeader(&hdr); err != nil {
			t.Fatal(err)
		}
		if len(bodies[i]) > 0 {
			if _, err := tw.Write(bodies[i]); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestSyncOnce_ExtractsContentAndHonorsETag304(t *testing.T) {
	payload := buildTarGz(t, []tar.Header{
		{Name: "nacfson-blog-main/content/posts/hello.md", Mode: 0o644, Size: 5, Typeflag: tar.TypeReg},
	}, [][]byte{[]byte("hello")})

	etag := `"rev-111"`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", etag)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(payload)
	}))
	defer srv.Close()

	workDir := t.TempDir()
	rel := &mockReloader{}
	s := syncer.NewFluxTarballSyncer(srv.URL, workDir, rel)

	updated, err := s.SyncOnce(context.Background())
	if err != nil || !updated {
		t.Fatalf("first sync: expected updated=true, err=%v", err)
	}
	got, err := os.ReadFile(filepath.Join(s.ActiveContentDir(), "posts", "hello.md"))
	if err != nil || string(got) != "hello" {
		t.Fatalf("unexpected extracted file: %q (err=%v)", string(got), err)
	}
	if rel.count.Load() != 1 {
		t.Fatalf("expected 1 reload call, got %d", rel.count.Load())
	}

	updated, err = s.SyncOnce(context.Background())
	if err != nil || updated {
		t.Fatalf("second sync with matching ETag: expected updated=false, err=%v", err)
	}
}

func TestSyncOnce_RejectsZipSlipAndSymlinks(t *testing.T) {
	zipSlip := buildTarGz(t, []tar.Header{
		{Name: "repo/content/../../evil.txt", Mode: 0o644, Size: 4, Typeflag: tar.TypeReg},
	}, [][]byte{[]byte("evil")})

	symlinkTar := buildTarGz(t, []tar.Header{
		{Name: "repo/content/posts/link.md", Mode: 0o777, Typeflag: tar.TypeSymlink, Linkname: "/etc/passwd"},
	}, [][]byte{nil})

	for name, archive := range map[string][]byte{"zip-slip": zipSlip, "symlink": symlinkTar} {
		t.Run(name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("ETag", `"bad"`)
				_, _ = w.Write(archive)
			}))
			defer srv.Close()

			s := syncer.NewFluxTarballSyncer(srv.URL, t.TempDir(), &mockReloader{})
			if _, err := s.SyncOnce(context.Background()); err == nil {
				t.Fatalf("%s: expected error, got nil", name)
			}
		})
	}
}

func TestSyncOnce_PreservesActiveContentOnCorruptTarball(t *testing.T) {
	seedDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(seedDir, "posts"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(seedDir, "posts", "seed.md"), []byte("seed-ok"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("ETag", `"corrupt"`)
		_, _ = w.Write([]byte("not-a-valid-gzip-stream"))
	}))
	defer srv.Close()

	s := syncer.NewFluxTarballSyncer(srv.URL, t.TempDir(), &mockReloader{})
	if err := s.SeedIfEmpty(seedDir); err != nil {
		t.Fatalf("SeedIfEmpty: %v", err)
	}
	if _, err := s.SyncOnce(context.Background()); err == nil {
		t.Fatalf("expected error on corrupt gzip")
	}
	got, err := os.ReadFile(filepath.Join(s.ActiveContentDir(), "posts", "seed.md"))
	if err != nil || string(got) != "seed-ok" {
		t.Fatalf("expected seed content preserved after failed sync, got %q (err=%v)", string(got), err)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/syncer/...`
Expected: FAIL (`cannot find package` / `undefined: syncer.NewFluxTarballSyncer`)

- [ ] **Step 3: Implement `internal/syncer/flux_tarball.go`**

Implement `FluxTarballSyncer` with:
- `SeedIfEmpty(seedDir string) error`: copies `seedDir` into `filepath.Join(workspaceDir, "seed")` and sets the active directory pointer (`atomic.Value` / `sync.RWMutex`) if not yet initialized.
- `SyncOnce(ctx context.Context) (bool, error)`: sends `GET` with `If-None-Match` (`10s` client timeout); on `200 OK`, extracts entries whose path contains `/content/` or starts with `content/` into a staging directory `filepath.Join(workspaceDir, fmt.Sprintf("rev-%d", time.Now().UnixNano()))`; rejects any `tar.Header` where `Typeflag` is not `tar.TypeReg` or `tar.TypeDir`, rejects any cleaned relative path starting with `..`, enforces `10 MiB` per file and `50 MiB` total via `io.LimitReader`; atomically updates the active directory pointer, calls `reloader.Reload()`, and removes the previous staging directory.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./internal/syncer/...`
Expected: PASS (`TestSyncOnce_ExtractsContentAndHonorsETag304`, `TestSyncOnce_RejectsZipSlipAndSymlinks`, `TestSyncOnce_PreservesActiveContentOnCorruptTarball`)

- [ ] **Step 5: Commit**

```bash
git add internal/syncer/
git commit -m "feat(syncer): add Flux source-controller tarball syncer with zip-slip guard"
```

---

### Task 3: Go 1.22 HTTP Server & Binary Entrypoint (`internal/server/`, `cmd/blog-server/`)

**Files:**
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/internal/server/router.go`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/cmd/blog-server/main.go`
- Test: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/internal/server/router_test.go`

**Interfaces:**
- Consumes: `content.ContentStore`, `content.ErrPostNotFound`, `content.ErrInvalidSlug`, `syncer.NewFluxTarballSyncer`
- Produces (`package server`):
  - `func NewHandler(store content.ContentStore, activeContentDir func() string, webOutDir string) http.Handler`

- [ ] **Step 1: Write failing HTTP router tests in `internal/server/router_test.go`**

```go
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
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/server/...`
Expected: FAIL (`cannot find package` / `undefined: server.NewHandler`)

- [ ] **Step 3: Implement `internal/server/router.go` and `cmd/blog-server/main.go`**

Implement `NewHandler` in `internal/server/router.go` using Go 1.22 `http.NewServeMux()` patterns:
- `GET /healthz` -> `200 OK` `{"status":"ok"}`
- `GET /api/posts` -> JSON list from `store.ListPosts(r.Context(), r.URL.Query().Get("pillar"), r.URL.Query().Get("tag"))`
- `GET /api/posts/{slug}` -> JSON post from `store.GetPost(r.Context(), r.PathValue("slug"))` (`400` on `ErrInvalidSlug`, `404` on `ErrPostNotFound`)
- `GET /api/competencies` -> raw JSON from `store.GetCompetencyMatrix(r.Context())`
- `GET /content/assets/{filepath...}` -> validates clean relative path inside `filepath.Join(activeContentDir(), "assets")`, sets `Cache-Control: public, max-age=300`
- `GET /content/demos/{filepath...}` -> validates clean relative path inside `filepath.Join(activeContentDir(), "demos")`, sets `Content-Security-Policy: default-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'none'; frame-ancestors 'self'` and `X-Content-Type-Options: nosniff`
- `GET /{path...}` -> serves static files from `webOutDir` (falling back to `post/index.html` for `/post` and `index.html` for `/` SPA routes).

Implement `cmd/blog-server/main.go` reading `PORT` (default `"8080"`), `CONTENT_WORKSPACE_DIR` (default `"/var/run/blog-content"`), `SEED_CONTENT_DIR` (default `"/app/seed-content"`), `WEB_OUT_DIR` (default `"/app/web/out"`), `FLUX_ARTIFACT_URL` (optional; when set, runs `SyncOnce` on startup and `Start(ctx, pollInterval)`), and graceful `SIGINT`/`SIGTERM` shutdown.

- [ ] **Step 4: Run tests and verify binary builds**

Run: `go test -v -race ./... && go build -o /tmp/blog-server ./cmd/blog-server && rm -f /tmp/blog-server`
Expected: PASS (all tests in `internal/content`, `internal/syncer`, and `internal/server` pass; binary compiles cleanly)

- [ ] **Step 5: Commit**

```bash
git add internal/server/ cmd/blog-server/
git commit -m "feat(server): add Go 1.22 HTTP mux, demo CSP headers, and blog-server main"
```

---

### Task 4: Next.js Light-Theme UI & Kimi-Style Markdown Block Dispatcher (`web/`)

**Files:**
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/package.json`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/next.config.mjs`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/tsconfig.json`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/tailwind.config.ts`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/postcss.config.mjs`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/vitest.config.ts`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/lib/types.ts`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/components/BlogV2AnimatedHero.tsx`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/components/BlogV2MiniNav.tsx`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/components/blocks/BlogPicture.tsx`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/components/blocks/BlogIframeTabs.tsx`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/components/blocks/BenchmarkChart.tsx`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/components/blocks/BenchmarkTable.tsx`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/components/MarkdownDispatcher.tsx`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/app/globals.css`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/app/layout.tsx`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/app/page.tsx`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/app/post/page.tsx`
- Test: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/src/components/__tests__/MarkdownDispatcher.test.tsx`

**Interfaces:**
- Consumes: HTTP JSON endpoints from Task 3 (`GET /api/posts`, `GET /api/posts/{slug}`, `GET /api/competencies`)
- Produces: Static Next.js export at `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web/out` (`index.html`, `post/index.html`, `_next/static/...`)

- [ ] **Step 1: Scaffold `web/` configuration and write failing component tests in `web/src/components/__tests__/MarkdownDispatcher.test.tsx`**

Create `web/package.json` with exact pinned versions:
```json
{
  "name": "nacfson-blog-web",
  "version": "0.1.0",
  "private": true,
  "engines": {
    "node": ">=22.0.0"
  },
  "scripts": {
    "dev": "next dev",
    "build": "next build",
    "test": "vitest run"
  },
  "dependencies": {
    "next": "14.2.15",
    "react": "18.3.1",
    "react-dom": "18.3.1"
  },
  "devDependencies": {
    "@testing-library/jest-dom": "6.5.0",
    "@testing-library/react": "16.0.1",
    "@types/node": "22.7.5",
    "@types/react": "18.3.11",
    "@types/react-dom": "18.3.1",
    "@vitejs/plugin-react": "4.3.2",
    "autoprefixer": "10.4.20",
    "jsdom": "25.0.1",
    "postcss": "8.4.47",
    "tailwindcss": "3.4.13",
    "typescript": "5.5.4",
    "vitest": "2.1.3"
  }
}
```
Create `web/tsconfig.json` (`target: "ES2022"`, `lib: ["dom", "dom.iterable", "es2022"]`, `module: "esnext"`, `moduleResolution: "bundler"`, `jsx: "preserve"`, `strict: true`), `web/next.config.mjs` (`output: 'export'`, `images: { unoptimized: true }`), `web/vitest.config.ts` (`environment: 'jsdom'`), and `web/src/components/__tests__/MarkdownDispatcher.test.tsx`:

```tsx
import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect } from 'vitest';
import { MarkdownDispatcher } from '../MarkdownDispatcher';

const SAMPLE_MD = `
## 1. Overview

\`\`\`blogPicture
{
  "src": "/content/assets/cilium-vault-architecture.svg",
  "alt": "Cilium Architecture",
  "caption": "Figure 1: Wire-level proxy architecture"
}
\`\`\`

\`\`\`blogIframeTabs
{
  "mountStrategy": "click-or-center",
  "height": "480px",
  "tabs": [
    {
      "id": "sim",
      "label": "Packet Simulator",
      "src": "/content/demos/hubble-flow-sim.html",
      "poster": "/content/assets/wire-proxy-poster.svg",
      "caption": "Interactive flow simulator"
    }
  ]
}
\`\`\`

\`\`\`benchmarkTable
{
  "caption": "Table 1: Metrics",
  "columns": ["Metric", "Standard", "Ours"],
  "rows": [["Plaintext Secrets", "4", "**0**"]]
}
\`\`\`
`;

describe('MarkdownDispatcher', () => {
  it('renders headings, blogPicture, blogIframeTabs poster, and benchmarkTable', () => {
    render(<MarkdownDispatcher markdown={SAMPLE_MD} />);
    expect(screen.getByText('1. Overview')).toBeDefined();
    expect(screen.getByText(/Figure 1: Wire-level proxy architecture/i)).toBeDefined();
    expect(screen.getByText('Table 1: Metrics')).toBeDefined();

    // In click-or-center mode, iframe is not mounted until Launch button is clicked
    expect( document.querySelector('iframe') ).toBeNull();
    const launchBtn = screen.getByRole('button', { name: /Launch Interactive Demo/i });
    fireEvent.click(launchBtn);

    const iframe = document.querySelector('iframe');
    expect(iframe).not.toBeNull();
    expect(iframe?.getAttribute('sandbox')).toBe('allow-scripts');
    expect(iframe?.getAttribute('src')).toBe('/content/demos/hubble-flow-sim.html');
  });

  it('renders inline fallback banner when custom block JSON is malformed', () => {
    const broken = "```blogIframeTabs\n{ bad json \n```";
    render(<MarkdownDispatcher markdown={broken} />);
    expect(screen.getByText(/Invalid blogIframeTabs JSON block/i)).toBeDefined();
  });
});
```

- [ ] **Step 2: Install `web/` dependencies and run Vitest to verify it fails**

Run: `cd /home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web && npm install && npx vitest run`
Expected: FAIL (`Failed to resolve import "../MarkdownDispatcher"`)

- [ ] **Step 3: Implement the Kimi-style Light-Theme components and Next.js pages in `web/src/`**

Implement:
- `BlogV2AnimatedHero.tsx`: Renders the `scrollHero` layered parallax container with `IntersectionObserver` pause, "Interactive Layers" range sliders, and "Reset Layers" button.
- `BlogV2MiniNav.tsx`: Sticky left scroll-spy table of contents highlighting active `<h2>` headings.
- `blocks/BlogPicture.tsx`, `blocks/BlogIframeTabs.tsx` (`click-or-center` poster with "Launch Interactive Demo" button + `<iframe sandbox="allow-scripts">` + Fullscreen modal toggle), `blocks/BenchmarkChart.tsx` (grouped SVG bar chart), and `blocks/BenchmarkTable.tsx` (bold `**value**` best-metric badge).
- `MarkdownDispatcher.tsx`: Splits Markdown into fenced code blocks and prose sections; extracts `scrollHero` for the article hero; dispatches `blogPicture`, `blogIframeTabs`, `benchmarkChart`, and `benchmarkTable` inside a `try/catch` JSON parser that renders `<div role="alert">Invalid <lang> JSON block</div>` on syntax errors.
- `app/page.tsx` (portfolio header, 3 Content Pillar filter pills, Competency Matrix grid, and post cards linking to `/post?slug=...`) and `app/post/page.tsx` (fetches `/api/posts/{slug}`, renders `BlogV2AnimatedHero`, `BlogV2MiniNav`, and `MarkdownDispatcher`).

- [ ] **Step 4: Run Vitest unit tests and Next.js static export build**

Run: `cd /home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/web && npx vitest run && npm run build && ls -la out/index.html out/post/index.html`
Expected: PASS (2 Vitest tests pass; `next build` exports `out/index.html` and `out/post/index.html`)

- [ ] **Step 5: Commit**

```bash
git add web/
git commit -m "feat(web): add Light-Theme Next.js UI and Kimi-style Markdown block dispatcher"
```

---

### Task 5: Dual-Track Container Build (`Dockerfile` + `.github/workflows/publish-ghcr-image.yml`)

**Files:**
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/Dockerfile`
- Create: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/.github/workflows/publish-ghcr-image.yml`
- Test: `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/internal/server/dual_track_ci_test.go`

**Interfaces:**
- Consumes: `cmd/blog-server`, `web/`, `content/`
- Produces: Multi-stage `Dockerfile` (`UID 10001:10001`, port `8080`) and Track A GitHub Actions workflow `.github/workflows/publish-ghcr-image.yml`

- [ ] **Step 1: Write failing CI/Dockerfile invariant test in `internal/server/dual_track_ci_test.go`**

```go
package server_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDualTrackPipelineInvariants(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}

	wfBytes, err := os.ReadFile(filepath.Join(root, ".github", "workflows", "publish-ghcr-image.yml"))
	if err != nil {
		t.Fatalf("missing workflow file: %v", err)
	}
	wf := string(wfBytes)
	for _, required := range []string{`"web/**"`, `"cmd/**"`, `"internal/**"`, `"Dockerfile"`, `linux/amd64,linux/arm64`} {
		if !strings.Contains(wf, required) {
			t.Errorf("workflow missing required token %s", required)
		}
	}
	if strings.Contains(wf, `"content/**"`) {
		t.Errorf("workflow must NOT trigger Docker builds on content/** changes")
	}

	dfBytes, err := os.ReadFile(filepath.Join(root, "Dockerfile"))
	if err != nil {
		t.Fatalf("missing Dockerfile: %v", err)
	}
	df := string(dfBytes)
	for _, required := range []string{"10001:10001", "/app/blog-server", "/app/web/out", "/app/seed-content", "EXPOSE 8080"} {
		if !strings.Contains(df, required) {
			t.Errorf("Dockerfile missing required directive %q", required)
		}
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test -v ./internal/server -run TestDualTrackPipelineInvariants`
Expected: FAIL (`missing workflow file`)

- [ ] **Step 3: Create `Dockerfile` and `.github/workflows/publish-ghcr-image.yml`**

Create `/home/nacfson/Projects/nacfson_blog/.worktrees/feat-cncf-blog/Dockerfile` (Stage 1 `node:22-alpine` builds `/work/web/out`; Stage 2 `golang:1.22-alpine` builds `CGO_ENABLED=0 /out/blog-server`; Stage 3 `alpine:3.20` copies `ca-certificates`, `/app/blog-server`, `/app/web/out`, `/app/seed-content`, sets `USER 10001:10001`, `EXPOSE 8080`, `ENTRYPOINT ["/app/blog-server"]`) and `.github/workflows/publish-ghcr-image.yml` matching Section 5.4 of the design spec.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test -v -race ./...`
Expected: PASS (`TestDualTrackPipelineInvariants` and all package tests pass)

- [ ] **Step 5: Commit**

```bash
git add Dockerfile .github/workflows/publish-ghcr-image.yml internal/server/dual_track_ci_test.go
git commit -m "ci(docker): add multi-stage UID 10001 Dockerfile and path-filtered GHCR workflow"
```

---

### Task 6: `nacfson_pipeline` Unified FluxCD & `proj-blog` Manifests

**Files (in `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog`):**
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/clusters/local-k3s/flux-system/gotk-sync.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/clusters/vps-k3s/flux-system/gotk-sync.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/boundary/kustomization.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/boundary/namespace.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/boundary/sign-in.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/boundary/service-account.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/boundary/reconciler-rbac.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/boundary/resource-quota.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/boundary/network-policy.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/workloads/kustomization.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/workloads/deployment.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/workloads/services.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/projects/blog/workloads/ingress-route.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/platform/governance/manual-change-policy.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/platform/governance/base-network-policy.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/platform/tls/certificates.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/deploy/platform/tls/tls-store.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/clusters/local-k3s/layers.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/clusters/vps-k3s/layers.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/clusters/local-k3s/projects/proj-blog.yaml`
- Create: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/clusters/vps-k3s/projects/proj-blog.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/clusters/local-k3s/projects/kustomization.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/clusters/vps-k3s/projects/kustomization.yaml`
- Modify: `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/scripts/render-invariants.py:119-130`

**Interfaces:**
- Consumes: `nacfson_pipeline` platform contracts (`CONTRACT_AUTH_DOOR.md`, `CONTRACT_ISOLATION_BUDGET.md`, `CONTRACT_TLS_INGRESS.md`, `CONTRACT_PROBES_READINESS.md`)
- Produces: Validated FluxCD and Kubernetes manifests for `proj-blog` passing `scripts/render-validate.sh` and `scripts/platform-preflight.py` across `local-k3s` and `vps-k3s`.

- [ ] **Step 1: Add `Namespace/proj-blog` requirement to `scripts/render-invariants.py` and verify render-validate fails before manifests exist**

In `/home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog/scripts/render-invariants.py` (line 125), add `("Namespace", "proj-blog", "kustomize.toolkit.fluxcd.io/prune", "disabled")` to `required_annotations`.

- [ ] **Step 2: Run `render-validate.sh` to verify it fails on missing `Namespace/proj-blog`**

Run: `cd /home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog && ./scripts/render-validate.sh local-k3s`
Expected: FAIL (`Invariant 5 violation: Missing required annotation kustomize.toolkit.fluxcd.io/prune: disabled on Namespace/proj-blog`)

- [ ] **Step 3: Create `deploy/projects/blog/{boundary,workloads}` and update Flux/TLS/Governance manifests**

1. **`clusters/{local-k3s,vps-k3s}/flux-system/gotk-sync.yaml`**: Append `GitRepository/nacfson-blog-content` (`namespace: flux-system`, `url: https://github.com/nacfson/nacfson_blog`, `interval: 1m`, `ref.branch: main`, `ignore: /*\n!/content/`).
2. **`deploy/projects/blog/boundary/`**:
   - `namespace.yaml`: `Namespace/proj-blog` with `pod-security.kubernetes.io/enforce: restricted` and `kustomize.toolkit.fluxcd.io/prune: disabled`.
   - `sign-in.yaml`: `ConfigMap/blog-sign-in` (`namespace: proj-blog`, `data.signIn: "none"`).
   - `service-account.yaml`: `ServiceAccount/blog-workload-sa` (`namespace: proj-blog`, `automountServiceAccountToken: false`).
   - `reconciler-rbac.yaml`: `ServiceAccount/blog-reconciler`, `Role/blog-reconciler`, `RoleBinding/blog-reconciler` in `proj-blog` (mirroring `deploy/projects/health/boundary/reconciler-rbac.yaml`).
   - `resource-quota.yaml`: `ResourceQuota/project-blog-quota` in `proj-blog` (`requests.cpu: "600m"`, `limits.cpu: "600m"`, `requests.memory: "512Mi"`, `limits.memory: "512Mi"`, `pods: "4"`, `services.nodeports: "0"`, `services.loadbalancers: "0"`).
   - `network-policy.yaml`: `NetworkPolicy/project-blog-isolation` in `proj-blog` allowing Ingress on port `8080` from `namespaceSelector: {name: platform}` and `kube-system/traefik`, and Egress to `kube-system/kube-dns` (UDP/TCP `53`) and `flux-system/source-controller` (TCP `80`, `9090`).
3. **`deploy/projects/blog/workloads/`**:
   - `deployment.yaml`: `Deployment/blog-web` in `proj-blog` (1 replica, `maxSurge: 1`, `serviceAccountName: blog-workload-sa`, `automountServiceAccountToken: false`, `runAsUser: 10001`, `runAsGroup: 10001`, `readOnlyRootFilesystem: true`, `capabilities.drop: ["ALL"]`, `requests == limits` at `cpu: "100m"`, `memory: "128Mi"`, env `FLUX_ARTIFACT_URL=http://source-controller.flux-system.svc.cluster.local:80/gitrepository/flux-system/nacfson-blog-content/latest.tar.gz`, `emptyDir: {}` volumes for `blog-content` at `/var/run/blog-content` and `tmp` at `/tmp`, `livenessProbe` and `readinessProbe` on `GET /healthz` port `8080`).
   - `services.yaml`: `Service/blog-web` (`ClusterIP`, port `8080` -> `targetPort: 8080`).
   - `ingress-route.yaml`: `IngressRoute/project-blog-ingress` in `proj-blog` (`entryPoints: [websecure]`, `tls: {}`, route `Host(\`blog.example.com\`)` with middleware `platform/strip-forged-headers` -> `blog-web:8080`).
4. **Platform TLS, Governance & Layer Wiring**:
   - `deploy/platform/tls/certificates.yaml` & `tls-store.yaml`: Add `Certificate/blog-tls` (`secretName: blog-tls`, `dnsNames: [blog.example.com]`) and list `secretName: blog-tls` in `TLSStore/default`.
   - `deploy/platform/governance/manual-change-policy.yaml`: Add `request.userInfo.username == 'system:serviceaccount:proj-blog:blog-reconciler'` to both `ValidatingAdmissionPolicy` expressions and add `- proj-blog` to `ValidatingAdmissionPolicyBinding/gitops-managed-namespaces`.
   - `deploy/platform/governance/base-network-policy.yaml`: Append `NetworkPolicy/allow-blog-content-artifact` in `namespace: flux-system` allowing Ingress to `podSelector: {matchLabels: {app: source-controller}}` on TCP `9090` from `namespaceSelector: {matchLabels: {kubernetes.io/metadata.name: proj-blog}}`.
   - `clusters/{local-k3s,vps-k3s}/layers.yaml`: Add `Kustomization/project-boundaries-blog` (`path: ./deploy/projects/blog/boundary`), add `project-boundaries-blog` to `dependsOn` of `tls` and `projects`, and in `clusters/vps-k3s/layers.yaml` patch `Certificate/blog-tls` to `blog.nacfson.cloud` / `letsencrypt-prod`.
   - `clusters/{local-k3s,vps-k3s}/projects/proj-blog.yaml` & `kustomization.yaml`: Add `Kustomization/proj-blog` (`namespace: proj-blog`, `serviceAccountName: blog-reconciler`, `path: ./deploy/projects/blog/workloads`, patching `IngressRoute/project-blog-ingress` on `vps-k3s` to `Host(\`blog.nacfson.cloud\`)`).

- [ ] **Step 4: Run `render-validate.sh` and `platform-preflight.py` for both `local-k3s` and `vps-k3s`**

Run:
```bash
cd /home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog
R_LOCAL=$(./scripts/render-validate.sh local-k3s)
python3 ./scripts/platform-preflight.py --environment local-k3s --candidate "$R_LOCAL" --capacity clusters/local-k3s/capacity.yaml
rm -f "$R_LOCAL"
R_VPS=$(./scripts/render-validate.sh vps-k3s)
python3 ./scripts/platform-preflight.py --environment vps-k3s --candidate "$R_VPS" --capacity clusters/vps-k3s/capacity.yaml
rm -f "$R_VPS"
```
Expected: PASS (`All render invariants passed successfully` and `Platform Preflight Status: PASSED` with `Project Count: 3` for both `local-k3s` and `vps-k3s`)

- [ ] **Step 5: Commit in `nacfson_pipeline`**

```bash
cd /home/nacfson/Projects/nacfson_pipeline/.worktrees/feat-proj-blog
git add clusters/ deploy/ scripts/render-invariants.py
git commit -m "feat(proj-blog): onboard proj-blog tenant and unified Flux content GitRepository"
```
