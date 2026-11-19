# nacfson_blog

Cloud-native engineering blog and interactive portfolio hosted at [blog.nacfson.cloud](https://blog.nacfson.cloud).

Built with a zero-dependency Go 1.22 standard-library server, a monochrome Next.js static-export reader with a WebGL1 ASCII hero and custom interactive Markdown blocks, FluxCD GitOps content hot-reloading, and a local-only git-worktree-isolated writing studio (**Blog Studio**).

---

## Architecture

```
                         ┌───────────────────────────────────────────┐
                         │        cmd/blog-server (Go 1.22)          │
                         │                                           │
  Browser ──HTTP────────▶│  internal/server (Router + RequestLogger) │
                         │    ├── /healthz                           │
                         │    ├── /api/posts, /api/posts/{slug}      │
                         │    ├── /content/assets/*                  │
                         │    ├── /content/demos/* (CSP + nosniff)   │
                         │    └── /*  (web/out Next.js SPA export)   │
                         │               │                           │
                         │               ▼                           │
                         │  internal/content (FolderStore)           │
                         │               ▲  Reload()                 │
                         │               │                           │
  Flux source-controller │  internal/syncer (FluxTarballSyncer)      │
  & Kubernetes API ─────▶│    ├── GitRepository artifact resolver    │
                         │    ├── ETag / rev-* validated extraction  │
                         │    └── Rollback-safe active root swap     │
                         └───────────────────────────────────────────┘

  Local Authoring Only (never packaged in Docker):
  scripts/open-studio.sh ─▶ cmd/blog-studio ─▶ internal/studio + studio/ (Vite/React/CodeMirror 6)
                                                └── per-draft .worktrees/studio-<key> + non-force push
```

### Core Pillars

- **Zero-Dependency Go 1.22 Server (`cmd/blog-server`, `internal/`)**: Serves the JSON post API, static assets, CSP-sandboxed interactive HTML demos, and the Next.js static export using only the Go standard library (`go.mod` has zero external dependencies). Logs structured JSON with W3C `traceparent` / `trace_id` propagation via `internal/logx`.
- **Dual-Track GitOps Content Hot-Reload (`internal/syncer`)**: Updates `content/**` without rebuilding container images or restarting pods. Resolves the current tarball URL and revision from an in-cluster FluxCD `GitRepository` (`FLUX_GITREPOSITORY`) or polls a direct artifact URL (`FLUX_ARTIFACT_URL`) using `ETag` / `If-None-Match`, rejecting symlinks, non-regular entries, zip-slip paths, and oversized archives (`10 MiB` per file / `50 MiB` total) before atomically swapping the active `rev-*` directory and reloading `FolderStore` (with automatic rollback on reload failure).
- **Monochrome Editorial Frontend (`web/`)**: Next.js `14.2.15` (`output: 'export'`) UI styled with a monochrome Tailwind stone palette (`#fafaf9` canvas, `#ffffff` cards, `#0c0a09` ink). Includes:
  - A plain-WebGL1 3-pass ASCII torus-knot hero (`AsciiHero` + `web/src/lib/ascii/`) with a static ASCII fallback when WebGL is unavailable and `prefers-reduced-motion` support.
  - A fenced-block Markdown dispatcher (`MarkdownDispatcher`) supporting `scrollHero`, `blogPicture`, `blogIframeTabs` (lazy click-or-center poster guard + `<iframe sandbox="allow-scripts">`), `benchmarkChart`, `benchmarkTable`, sticky `BlogV2MiniNav` scroll-spy, and isolated per-block JSON error fallback.
- **Local-Only Blog Studio (`cmd/blog-studio`, `internal/studio`, `studio/`)**: Loopback-only, git-backed writing interface with live production-parity preview, CodeMirror 6 editor, custom-block insertion, asset/demo uploads and linting, and git activity SSE streaming. Every draft lives on its own `studio/<key>` branch inside `.worktrees/studio-<key>/`; publishing squashes onto a detached `.worktrees/studio-publish/` worktree, verifies only `content/**` paths are touched and all posts pass `internal/content.ValidateDir`, commits `content: publish <slug>`, and pushes without `--force`.

---

## Repository Layout

| Path | Role |
| :--- | :--- |
| `cmd/blog-server/` | Production `blog-web` HTTP server entrypoint |
| `cmd/blog-studio/` | Local-only Blog Studio CLI entrypoint |
| `internal/content/` | Post domain types (`Topic`, `PostSummary`, `Post`), thread-safe `FolderStore`, and `ValidatePost` / `ValidateDir` |
| `internal/logx/` | Structured JSON `log/slog` handler, exception normalization, bounded stack traces, and W3C `traceparent` helpers |
| `internal/server/` | `http.ServeMux` router, path-traversal guards, demo CSP headers, static SPA fallback, and `RequestLogger` middleware |
| `internal/studio/` | Blog Studio loopback HTTP/SSE server, draft worktree manager, frontmatter serializer, linter, PID lock, and publisher |
| `internal/syncer/` | FluxCD `GitRepository` artifact resolver and validated tarball syncer |
| `web/` | Next.js `14.2.15` static-export blog frontend + Vitest test suite |
| `studio/` | Vite + React + CodeMirror 6 frontend embedded into `bin/blog-studio` |
| `content/` | Published Markdown posts (`posts/`), static images (`assets/`), and sandboxed HTML demos (`demos/`) |
| [`scripts/`](./scripts/README.md) | Developer convenience launchers (`open-studio.sh`) |
| `tests/` | Content fixtures and cross-agent skill verification tests |
| `docs/` | Architecture design specs, plans, and mockups |
| [`Makefile`](./Makefile) | Build and test targets for Blog Studio (`studio`, `studio-ui`, `studio-test`) |
| [`Dockerfile`](./Dockerfile) | Multi-stage build producing the non-root (`10001:10001`) production `blog-server` image |

---

## Quick Start

### Prerequisites

- **Go** `1.22+`
- **Node.js** `22+` and `npm`

### Run the Production Blog Server Locally

1. Build the Next.js static export:
   ```bash
   (cd web && npm ci && npm run build)
   ```
2. Start `blog-server` pointing at the local `content/` and `web/out/` directories:
   ```bash
   CONTENT_WORKSPACE_DIR=/tmp/nacfson-blog-workspace \
   SEED_CONTENT_DIR=./content \
   WEB_OUT_DIR=./web/out \
   PORT=8080 \
   go run ./cmd/blog-server
   ```
3. Open `http://localhost:8080`.

#### Server Environment Variables (`cmd/blog-server`)

| Variable | Default | Description |
| :--- | :--- | :--- |
| `PORT` | `8080` | TCP port the HTTP server binds to |
| `CONTENT_WORKSPACE_DIR` | `/var/run/blog-content` | Writable directory for `seed/` and extracted `rev-*` revisions |
| `SEED_CONTENT_DIR` | `/app/seed-content` | Initial content directory copied if workspace is empty |
| `WEB_OUT_DIR` | `/app/web/out` | Next.js static export directory (`web/out`) |
| `FLUX_GITREPOSITORY` | `""` | In-cluster Flux `GitRepository` ref (`<namespace>/<name>`) to resolve artifact URLs dynamically |
| `FLUX_ARTIFACT_URL` | `""` | Direct Flux tarball artifact URL (used when `FLUX_GITREPOSITORY` is unset) |
| `FLUX_POLL_INTERVAL` | `60s` | Positive Go duration between artifact polls (`syncer.DefaultPollInterval`) |

### HTTP Endpoints (`blog-server`)

| Method & Path | Description |
| :--- | :--- |
| `GET /healthz` | Health probe returning `{"status":"ok"}` |
| `GET /api/posts?topic=<topic>` | Returns JSON `[]PostSummary` sorted newest-first, optionally filtered by topic |
| `GET /api/posts/{slug}` | Returns JSON `Post` (metadata + `markdownBody`); `400` on invalid slug, `404` if missing |
| `GET /content/assets/{filepath...}` | Serves files from active `assets/` with `Cache-Control: public, max-age=300` |
| `GET /content/demos/{filepath...}` | Serves files from active `demos/` with `Content-Security-Policy` (`connect-src 'none'`, `frame-ancestors 'self'`) and `X-Content-Type-Options: nosniff` |
| `GET /{path...}` | Serves `web/out` static assets with SPA fallback to `/post/index.html` and `/index.html` |

---

## Authoring with Blog Studio

Blog Studio is a local-only writing platform that never leaves `127.0.0.1` and is never included in the production Docker image.

```bash
# Quick launch (builds bin/blog-studio if missing, opens browser, runs on 127.0.0.1:4321)
./scripts/open-studio.sh

# Or build and run explicitly with custom flags
make studio
./bin/blog-studio --repo . --addr 127.0.0.1:4321 --publish-branch main --remote origin
```

See [`scripts/README.md`](./scripts/README.md) for launcher environment variables (`STUDIO_ADDR`, `STUDIO_PUBLISH_BRANCH`).

---

## Container Build

The multi-stage [`Dockerfile`](./Dockerfile) builds `web/out` (Node 22 Alpine) and a static `CGO_ENABLED=0` `blog-server` binary (Go 1.22 Alpine), then copies both plus `content/` (as `/app/seed-content`) into a minimal Alpine 3.20 runtime image running as non-root UID/GID `10001:10001` on port `8080`:

```bash
docker build -t nacfson-blog:local .
docker run --rm -p 8080:8080 nacfson-blog:local
```

---

## Verification & Testing

```bash
# Go unit, race, and dual-track CI contract tests
go test -v -race ./... && go vet ./...

# Blog frontend unit tests and static export build
(cd web && npm test && npm run build)

# Blog Studio backend + frontend tests
make studio-test

# Skill tests and automated headless Playwright UI verification
python3 -m unittest discover -s tests/skills
python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py --mode verify
```
