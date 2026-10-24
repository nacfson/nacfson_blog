# `nacfson_blog` (`proj-blog`) — Interactive Engineering Blog & Portfolio Design Specification

- **Date**: 2026-10-24
- **Status**: Draft (Pending User Approval)
- **Target Repositories**:
  - Application & Content Repository: `/home/nacfson/Projects/nacfson_blog` (`github.com/nacfson/nacfson_blog`)
  - Platform GitOps Repository: `/home/nacfson/Projects/nacfson_pipeline` (`github.com/nacfson/nacfson_pipeline`)
- **Tenant Namespace**: `proj-blog`
- **Public Hostname**: `https://blog.nacfson.cloud`

---

## 1. Intent and Why

`nacfson_blog` is an interactive, frontier-research-style engineering blog and technical portfolio hosted as tenant project `proj-blog` inside the existing K3s platform (`nacfson_pipeline`).

The blog serves three core content pillars:
1. **Project Architecture Updates**: Deep architectural walkthroughs and milestone updates for `nacfson_pipeline` (Zero-Plaintext OpenBao proxies, Cilium L7 DNS/FQDN policies, FluxCD reconciliation layers), `Project_PN`, and future platform tenants.
2. **Technical Competency & Skill Growth Matrix**: Structured tracking of systems, Kubernetes/CNCF, security, and backend engineering competencies linked directly to verifiable code artifacts and posts.
3. **Technical Opinions & Deep Dives**: Interactive engineering analyses of recent technologies, complete with live architecture simulators, SVG diagrams, and benchmark comparisons.

### Visual & Authoring Inspiration
Inspired by frontier AI technical blogs (`https://www.kimi.ai/blog/kimi-k3-1-pro`), articles are authored as standard Git-tracked Markdown (`.md`) files in `content/posts/` that embed **custom fenced JSON blocks** (`scrollHero`, `blogPicture`, `blogIframeTabs`, `benchmarkChart`, `benchmarkTable`). The frontend renders these blocks in a clean **Light Theme** editorial layout with a layered scroll-parallax hero, sticky scroll-spy table of contents (`BlogV2MiniNav`), and poster-to-live-HTML (`click-or-center`) interactive sandboxes.

---

## 2. Goals, Non-Goals, and Anti-Goals

### 2.1 Goals
- **Single-Pod Tenant Deployment (`blog-web`)**: Run the entire blog (Next.js UI + Go 1.22 HTTP server + live-synced content directory) in **1 single pod** inside `namespace: proj-blog` with a minimal resource footprint (`requests == limits`, e.g., `100m` CPU, `128Mi` RAM).
- **Public Read-Only Door (`signIn: none`)**: Comply with `nacfson_pipeline/contracts/CONTRACT_AUTH_DOOR.md` using `signIn: none` so visitors can read articles without a Keycloak login wall while `strip-forged-headers` protects the ingress route.
- **GitOps Folder Mode (`STORAGE_MODE=folder`) Behind `ContentStore`**: Store all posts (`content/posts/*.md`), diagrams (`content/assets/*`), interactive HTML demos (`content/demos/*.html`), and competency metadata (`content/competencies/matrix.json`) directly in Git behind a clean Go `ContentStore` interface so a future database adapter (`STORAGE_MODE=db`) can be added without touching UI or HTTP handler code.
- **Unified FluxCD Zero-Image-Rebuild Content Sync**: Reuse the cluster's **single existing `flux-system` (`source-controller`)** to watch `github.com/nacfson/nacfson_blog` with `spec.ignore: /* !/content/`. When a new `.md` post, `.svg` diagram, or `.html` demo is pushed to `content/**`, GitHub Actions skips the Docker image build (`paths: ['web/**', 'cmd/**', 'Dockerfile']`), and `blog-web` pulls the updated `.tar.gz` artifact from `source-controller` over internal ClusterIP into a writable `emptyDir` volume (`/var/run/blog-content/`) in ~30 seconds with **zero Docker image rebuild and zero pod restart**.
- **Light-Theme Kimi-Style Interactive Article Experience**: Support `scrollHero` (layered scroll/drag parallax header), `blogPicture` (responsive SVG/WebP figure with caption), `blogIframeTabs` (tabbed interactive HTML `<iframe sandbox="allow-scripts">` with `click-or-center` poster lazy-mounting and fullscreen modal), `benchmarkChart` (grouped SVG bar chart), `benchmarkTable` (metric comparison table with bold best-value highlighting), and `BlogV2MiniNav` (sticky scroll-spy section navigation).

### 2.2 Non-Goals
- **Public User Accounts or Comments**: No visitor login, registration, or comment threads.
- **Web Admin WYSIWYG Editor**: Authoring happens through Git commits (`git push`) of Markdown, SVG, and HTML files.
- **PostgreSQL Provisioning in v1**: No database role, OpenBao `vault-db-proxy` connection, or PVC is provisioned for `proj-blog` in v1.

### 2.3 Anti-Goals
- **No Container Image Rebuilds for Content Changes**: Pushing a new `.md` post or fixing a typo in `content/**` must **never** trigger a Docker image build or push a new image layer to `ghcr.io`.
- **No Second Flux Installation or Sidecar Containers**: Do not install a second GitOps operator or run a `git-sync` sidecar container inside `blog-web`.
- **No Relaxation of `nacfson_pipeline` Security Invariants**: Never weaken `restricted` Pod Security Admission, `runAsUser: 10001`, `readOnlyRootFilesystem: true`, `capabilities.drop: ["ALL"]`, or `automountServiceAccountToken: false`.

---

## 3. Platform Constraints (`nacfson_pipeline` Contract Alignment)

Every manifest for `proj-blog` must pass `scripts/platform-preflight.py`, `scripts/render-invariants.py`, and `scripts/render-validate.sh` in `nacfson_pipeline`:

| Contract / Script | Requirement | `proj-blog` Design |
| :--- | :--- | :--- |
| **`CONTRACT_AUTH_DOOR.md`** | `signIn` must be `shared` or `none`; route must include `strip-forged-headers` and must not include `forward-auth`; `signIn: none` forbids egress to `identity/gateway`. | `deploy/projects/blog/boundary/sign-in.yaml` sets `signIn: none`. `IngressRoute` applies `platform/strip-forged-headers`. Zero auth env vars or `identity` egress. |
| **`CONTRACT_ISOLATION_BUDGET.md`** | `restricted` PSS on namespace; `runAsNonRoot: true`, `runAsUser: 10001`, `runAsGroup: 10001`, `readOnlyRootFilesystem: true`, `allowPrivilegeEscalation: false`, `capabilities.drop: ["ALL"]`, `seccompProfile: RuntimeDefault`, `automountServiceAccountToken: false`; strict QoS (`requests == limits`); no `hostPath` volumes. | `Deployment/blog-web` runs as `10001:10001` with `readOnlyRootFilesystem: true` and mounts `emptyDir: {}` at `/var/run/blog-content` and `/tmp`. `requests == limits` (`100m` CPU, `128Mi` memory). |
| **`CONTRACT_DATABASE.md` & `CONTRACT_PROBES_READINESS.md`** | `platform-preflight.py` (lines 392–394) only enforces `DATABASE_URL` and `/readyz` DB checks when a container declares DB env vars (`DATABASE_URL`, `DB_HOST`, `DB_NAME`, `DB_USER`, `DB_TOKEN`). | `blog-web` declares zero DB env vars and exposes `/healthz` (`200 OK` once initial content directory is ready) for both `livenessProbe` and `readinessProbe`. |
| **`CONTRACT_TLS_INGRESS.md`** | Platform owns all `Certificate` objects (`deploy/platform/tls`) and `TLSStore/default`. Project `IngressRoute` must use `entryPoints: [websecure]`, `tls: {}`, and explicit `Host(\`blog.nacfson.cloud\`)`. | Add `Certificate/blog-tls` (`secretName: blog-tls`, `dnsNames: [blog.nacfson.cloud]`) to `deploy/platform/tls` and `TLSStore/default`. |
| **`render-invariants.py` (Invariant 2)** | `GitRepository` objects in `clusters/<env>/` must use `https://` URLs and must not declare `secretRef`. | `GitRepository/nacfson-blog-content` in `flux-system` uses `url: https://github.com/nacfson/nacfson_blog` with no `secretRef`. |
| **`render-invariants.py` (Invariant 5 & 6)** | Project namespaces require `kustomize.toolkit.fluxcd.io/prune: disabled`. Project workloads may only render `{Deployment, Service, ConfigMap, IngressRoute}` and must use `serviceAccountName: blog-reconciler`. | `Namespace/proj-blog` carries `prune: disabled` (and is added to `required_annotations` in `render-invariants.py`). `Kustomization/proj-blog` uses `serviceAccountName: blog-reconciler`. |
| **`render-invariants.py` (Invariant 8)** | `ValidatingAdmissionPolicy/gitops-managed-namespaces` and `gitops-managed-cluster-kinds` must allowlist the project reconciler SA, and `ValidatingAdmissionPolicyBinding/gitops-managed-namespaces` must include every namespace in `deploy/`. | Add `proj-blog` and `system:serviceaccount:proj-blog:blog-reconciler` to `deploy/platform/governance/manual-change-policy.yaml`. |

---

## 4. Approaches Considered and Tradeoffs

### 4.1 Storage Architecture
- **Option A — Folder Mode Only (`STORAGE_MODE=folder`) with `ContentStore` Interface (Chosen)**:
  - *How it works*: Posts, SVGs, and HTML demos are plain files in `content/` read through a Go `ContentStore` interface (`ListPosts`, `GetPost`, `GetCompetencyMatrix`).
  - *Pros*: Zero database RAM/CPU overhead, full Git version history, offline Markdown authoring, trivial portability to any server, and zero-effort future upgrade to Postgres by adding a `PostgresContentStore` implementation.
  - *Cons*: No dynamic user-generated content (not needed for a single-author engineering blog).
- **Option B — PostgreSQL Database Mode (`STORAGE_MODE=db`)**:
  - *Why rejected for v1*: Requires `vault-db-proxy` wiring, schema migrations, and storing multi-file interactive HTML/SVG bundles as blobs or external URLs.

### 4.2 Frontend Stack
- **Option 2B — Next.js (App Router) + React + Tailwind CSS (Chosen)**:
  - *How it works*: Next.js App Router (Light Theme) compiled to static client assets (`output: 'export'` / SPA shell) embedded/served by the Go 1.22 server, paired with a React Markdown AST dispatcher for Kimi-style custom blocks (`scrollHero`, `blogPicture`, `blogIframeTabs`, `benchmarkChart`, `benchmarkTable`).
  - *Pros*: Matches `kimi.ai/blog` component architecture 1:1 while avoiding a runtime Node.js server process inside the production container.
- **Option 2A — Astro + React Islands**:
  - *Why not chosen*: Requires Astro build time whenever Markdown changes unless paired with client-side fetching anyway; User selected **2B (Next.js + React + Tailwind)**.

### 4.3 In-Cluster Server Runtime
- **Option 3A — Pure Go 1.22 Stdlib HTTP Server (`net/http`) (Chosen)**:
  - *How it works*: Single static Go binary (`CGO_ENABLED=0`) serving the compiled Next.js frontend bundle, `/api/posts*` JSON endpoints, `/content/*` static assets/demos, `/healthz`, and the background `FluxTarballSyncer`.
  - *Pros*: `<15 MiB` runtime memory, instant startup, native `archive/tar` + `compress/gzip` stdlib extraction, zero Node.js runtime vulnerabilities or memory bloat in K3s.

### 4.4 Live Content Delivery Without Image Rebuilds
- **Option 1 — Unified Flux `source-controller` Tarball + Pod `emptyDir` Sync (Chosen)**:
  - *How it works*: Flux's existing `source-controller` fetches `github.com/nacfson/nacfson_blog`, filters `/* !/content/`, and serves `latest.tar.gz` internally at `http://source-controller.flux-system.svc.cluster.local:80`. The Go server in `blog-web` checks the `ETag` every 30s and extracts updated files into `/var/run/blog-content/` (`emptyDir`).
  - *Pros*: Zero Docker rebuilds on new posts, bypasses Kubernetes' 1 MiB `ConfigMap` limit, requires zero external internet egress from `proj-blog`, and reuses the existing `flux-system` pod.
- **Option 2 — Kubernetes `ConfigMap` Volume via `kustomize-controller`**:
  - *Why rejected*: Kubernetes/etcd enforces a hard `1,048,576`-byte (1 MiB) limit per `ConfigMap`, which breaks after ~15 posts + SVGs + interactive HTML demos.
- **Option 3 — Rebuild Docker Image on Every Post**:
  - *Why rejected*: Wastes CI minutes, registry storage, and pod restarts for simple Markdown edits.

---

## 5. Detailed Architecture & Component Design

### 5.1 Repository Structure (`nacfson_blog`)

```text
nacfson_blog/
├── cmd/
│   └── blog-server/
│       └── main.go                 # Entrypoint: flags/env, ContentStore init, FluxTarballSyncer, HTTP mux
├── internal/
│   ├── content/
│   │   ├── store.go                # ContentStore interface + Post / Competency types
│   │   ├── folder_store.go         # FolderStore implementation (parses YAML frontmatter + Markdown)
│   │   └── folder_store_test.go    # Unit tests for frontmatter, sorting, pillar filtering, path traversal protection
│   ├── syncer/
│   │   ├── flux_tarball.go         # ETag poller + archive/tar + compress/gzip atomic extractor to emptyDir
│   │   └── flux_tarball_test.go    # Unit tests with httptest tarball server, zip-slip guard, ETag 304 handling
│   └── server/
│       ├── router.go               # Go 1.22 net/http mux (/api/posts, /api/competencies, /content/*, /healthz, SPA fallback)
│       └── router_test.go          # HTTP handler tests (CSP headers on /content/demos/*, 404s, JSON payloads)
├── web/                            # Next.js (App Router) + React + Tailwind CSS (Light Theme)
│   ├── package.json
│   ├── next.config.mjs             # output: 'export' into web/out
│   ├── tailwind.config.ts
│   └── src/
│       ├── app/
│       │   ├── layout.tsx          # Light-theme editorial shell + top navigation
│       │   ├── page.tsx            # Portfolio home: 3 Content Pillars filter + Competency Matrix + Post feed
│       │   └── post/
│       │       └── page.tsx        # Article view (/post?slug=...) with BlogV2AnimatedHero + BlogV2MiniNav + Markdown renderer
│       └── components/
│           ├── BlogV2AnimatedHero.tsx  # Layered scrollHero (base sky + parallax layers + Interactive/Reset toggle + offscreen pause)
│           ├── BlogV2MiniNav.tsx       # Sticky left scroll-spy TOC tracking <h2> headings
│           ├── MarkdownDispatcher.tsx  # Parses markdown & routes custom fenced JSON blocks to React components
│           └── blocks/
│               ├── BlogPicture.tsx     # Responsive SVG/WebP figure with zoom & caption
│               ├── BlogIframeTabs.tsx  # Tabbed interactive HTML sandbox (click-or-center poster -> live iframe + fullscreen)
│               ├── BenchmarkChart.tsx  # Grouped SVG bar chart with hover tooltips
│               └── BenchmarkTable.tsx  # Comparison table with automatic best-metric highlighting
├── content/                        # Live-synced by Flux source-controller (also baked as fallback seed)
│   ├── competencies/
│   │   └── matrix.json             # Competency domains, levels, verified skills, and linked post slugs
│   ├── posts/
│   │   └── 2026-10-24-cilium-zero-plaintext-k3s.md
│   ├── assets/
│   │   └── *.svg                   # Hero parallax layers & architecture diagrams
│   └── demos/
│       └── hubble-flow-sim.html    # Self-contained interactive HTML simulator
├── Dockerfile                      # Multi-stage build (Node 20 builds web/out -> Go 1.22 builds binary -> minimal UID 10001 runtime)
└── .github/
    └── workflows/
        └── build.yml               # Runs tests + builds GHCR image ONLY when web/**, cmd/**, internal/**, or Dockerfile change
```

### 5.2 Markdown Frontmatter & Custom Block Schema

Each post in `content/posts/<YYYY-MM-DD-slug>.md` starts with YAML frontmatter followed by standard GitHub-Flavored Markdown and optional custom fenced JSON code blocks:

```markdown
---
title: "Zero-Plaintext K3s: Wire-Level Vault Proxies & Cilium L7 Isolation"
slug: "cilium-zero-plaintext-k3s"
date: "2026-10-24"
pillar: "project-updates" # Enum: "project-updates" | "competency-growth" | "tech-opinions"
project: "nacfson_pipeline"
summary: "How we eliminated plaintext database and registry credentials from project namespaces using Go wire proxies and Cilium eBPF policies."
tags: ["Kubernetes", "Cilium", "OpenBao", "Go", "Zero-Trust"]
readTimeMinutes: 8
---
```

Supported custom fenced code block languages in `MarkdownDispatcher.tsx`:
1. **` ```scrollHero `**: Renders `BlogV2AnimatedHero` at the top of the article with `heightVh`, `baseImage`, `layers[]` (`src`, `alt`, `parallaxSpeed`, `initialOffsetY`), and `controls` (`allowLayerDrag`, `showParallaxSliders`). Pauses `requestAnimationFrame` via `IntersectionObserver` when scrolled out of view.
2. **` ```blogPicture `**: Renders `BlogPicture` with `src` (resolved against `/content/assets/…`), `alt`, `caption`, and `maxWidth`.
3. **` ```blogIframeTabs `**: Renders `BlogIframeTabs` with `mountStrategy` (`"click-or-center"` | `"eager"`), `height` (`"520px"`), and `tabs[]` (`id`, `label`, `src`, `poster`, `caption`). In `"click-or-center"` mode, renders the lightweight SVG poster until the user clicks "Launch Interactive Demo" or scrolls the block into the viewport center (`IntersectionObserver`), then mounts `<iframe sandbox="allow-scripts" src="/content/demos/...">` with a Fullscreen toggle.
4. **` ```benchmarkChart `**: Renders `BenchmarkChart` with `title`, `unit`, `metrics[]`, and `series[]` as a clean Light-Theme SVG bar chart.
5. **` ```benchmarkTable `**: Renders `BenchmarkTable` with `caption`, `columns[]`, and `rows[][]` (cells prefixed with `**` are highlighted as best-in-class).

### 5.3 Go 1.22 Stdlib Server & `ContentStore` Interface

```go
package content

import "context"

type Pillar string

const (
    PillarProjectUpdates   Pillar = "project-updates"
    PillarCompetencyGrowth Pillar = "competency-growth"
    PillarTechOpinions     Pillar = "tech-opinions"
)

type PostSummary struct {
    Slug            string   `json:"slug"`
    Title           string   `json:"title"`
    Date            string   `json:"date"`
    Pillar          Pillar   `json:"pillar"`
    Project         string   `json:"project,omitempty"`
    Summary         string   `json:"summary"`
    Tags            []string `json:"tags"`
    ReadTimeMinutes int      `json:"readTimeMinutes"`
}

type Post struct {
    PostSummary
    MarkdownBody string `json:"markdownBody"`
}

type ContentStore interface {
    ListPosts(ctx context.Context, pillar string, tag string) ([]PostSummary, error)
    GetPost(ctx context.Context, slug string) (*Post, error)
    GetCompetencyMatrix(ctx context.Context) ([]byte, error)
}
```

#### HTTP Endpoints (`internal/server/router.go`)
- `GET /healthz`: Returns `200 OK` (`{"status":"ok"}`). Used by Kubernetes `livenessProbe` and `readinessProbe`.
- `GET /api/posts?pillar=<pillar>&tag=<tag>`: Returns JSON array of `PostSummary` sorted newest-first by `Date`.
- `GET /api/posts/{slug}`: Returns JSON `Post` including raw `MarkdownBody` for `MarkdownDispatcher.tsx`.
- `GET /api/competencies`: Returns `content/competencies/matrix.json`.
- `GET /content/assets/{filepath...}`: Serves SVG/PNG/WebP files from `/var/run/blog-content/assets/` with path-traversal protection and `Cache-Control: public, max-age=300`.
- `GET /content/demos/{filepath...}`: Serves standalone interactive HTML files from `/var/run/blog-content/demos/` with strict isolation headers (`Content-Security-Policy: default-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'none'; frame-ancestors 'self'`, `X-Content-Type-Options: nosniff`).
- `GET /{path...}`: Serves the static Next.js frontend export from `/app/web/out`.

### 5.4 Zero-Image-Rebuild Content Sync (`internal/syncer/flux_tarball.go`)

1. **Bootstrap Seed on Startup**:
   - The container image includes a read-only seed copy of `content/` at `/app/seed-content/`.
   - On startup, before starting the HTTP listener, `blog-server` copies `/app/seed-content/` into `/var/run/blog-content/current` (`emptyDir`) if empty, then immediately attempts a one-shot sync from `FLUX_ARTIFACT_URL` (`http://source-controller.flux-system.svc.cluster.local:80/gitrepository/flux-system/nacfson-blog-content/latest.tar.gz`). Even if `source-controller` is momentarily unreachable during cold boot, the pod starts cleanly with the seed content and passes `/healthz`.
2. **Background Polling & Atomic Swap**:
   - Every `FLUX_POLL_INTERVAL` (default `30s`), `FluxTarballSyncer` sends an HTTP `GET` with `If-None-Match: <last-etag>` to `FLUX_ARTIFACT_URL`.
   - If `source-controller` returns `304 Not Modified`, zero bytes are downloaded.
   - If `source-controller` returns `200 OK`, `FluxTarballSyncer` streams the `.tar.gz` through Go stdlib `compress/gzip` and `archive/tar` into a staging directory `/var/run/blog-content/staging-<etag>/` (enforcing a max file size limit and rejecting any `..` path traversal entries), atomically swaps the active content pointer, deletes the old staging directory, and refreshes the in-memory `FolderStore` index.

### 5.5 Unified FluxCD & `nacfson_pipeline` Manifests

In `/home/nacfson/Projects/nacfson_pipeline`:

1. **`clusters/<env>/flux-system/gotk-sync.yaml`**:
   Add `GitRepository/nacfson-blog-content` in `namespace: flux-system`:
   ```yaml
   apiVersion: source.toolkit.fluxcd.io/v1
   kind: GitRepository
   metadata:
     name: nacfson-blog-content
     namespace: flux-system
   spec:
     interval: 1m
     url: https://github.com/nacfson/nacfson_blog
     ref:
       branch: main
     ignore: |
       /*
       !/content/
   ```
2. **`deploy/projects/blog/boundary/`**:
   - `namespace.yaml`: `Namespace/proj-blog` with `pod-security.kubernetes.io/enforce: restricted` and `kustomize.toolkit.fluxcd.io/prune: disabled`.
   - `sign-in.yaml`: `ConfigMap/blog-sign-in` with `data.signIn: "none"`.
   - `service-account.yaml`: `ServiceAccount/blog-sa` (`automountServiceAccountToken: false`).
   - `reconciler-rbac.yaml`: `ServiceAccount/blog-reconciler`, `Role/blog-reconciler`, and `RoleBinding/blog-reconciler` in `proj-blog`.
   - `resource-quota.yaml`: `ResourceQuota/project-blog-quota` sized to the environment slice.
   - `network-policy.yaml`:
     - `NetworkPolicy/project-blog-isolation` in `proj-blog`: allows Ingress on TCP `8080` from `platform` and `kube-system/traefik`; allows Egress to `kube-system/kube-dns` (UDP/TCP `53`) and `flux-system/source-controller` (TCP `80` and `9090`).
     - `NetworkPolicy/allow-blog-content-fetch` in `flux-system`: allows Ingress to `app: source-controller` on TCP `9090` from `namespaceSelector: {kubernetes.io/metadata.name: proj-blog}`.
3. **`deploy/projects/blog/workloads/`**:
   - `deployment.yaml`: `Deployment/blog-web` (1 replica, `UID 10001`, `readOnlyRootFilesystem: true`, `emptyDir` mounts at `/var/run/blog-content` and `/tmp`, `requests == limits` at `100m` CPU / `128Mi` memory, `/healthz` liveness & readiness probes on port `8080`).
   - `services.yaml`: `Service/blog-web` (`ClusterIP`, port `8080` -> `targetPort: 8080`).
   - `ingress-route.yaml`: `IngressRoute/project-blog-ingress` on `websecure` with `Host(\`blog.nacfson.cloud\`)`, middleware `platform/strip-forged-headers`, and `tls: {}`.
4. **Platform TLS & Governance Updates in `nacfson_pipeline`**:
   - Add `Certificate/blog-tls` (`blog.nacfson.cloud`) to `deploy/platform/tls/certificates.yaml`, `deploy/platform/tls/tls-store.yaml`, and `clusters/<env>/layers.yaml`.
   - Add `Kustomization/project-boundaries-blog` to `clusters/<env>/layers.yaml` and `clusters/<env>/projects/proj-blog.yaml` (`Kustomization/proj-blog`).
   - Add `proj-blog` and `system:serviceaccount:proj-blog:blog-reconciler` to `deploy/platform/governance/manual-change-policy.yaml` and `scripts/render-invariants.py`.

---

## 6. What's Left to the Builder

The following tactical implementation details are intentionally left to the implementation plan and builder:
- Exact Tailwind typography scale, spacing tokens, and light-theme color accents (`#f8fafc` canvas, `#ffffff` cards, `#2563eb` primary accent).
- Markdown parser library selection in `web/` (e.g., `react-markdown` + `remark-gfm` vs. custom tokenizer) as long as all 5 custom fenced JSON block types (`scrollHero`, `blogPicture`, `blogIframeTabs`, `benchmarkChart`, `benchmarkTable`) render identically to the validated sample (`samples/2026-10-24-cilium-zero-plaintext-k3s.md`).
- Exact CPU/memory adjustments in `nacfson_pipeline/deploy/projects/*/boundary/resource-quota.yaml` to ensure the 3-project (`proj-pn`, `proj-health`, `proj-blog`) peak footprint passes `scripts/platform-preflight.py` for both `local-k3s` and `vps-k3s`.
