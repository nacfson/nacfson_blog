---
name: local-ui-verify
description: Use when modifying frontend components, styles, custom Markdown blocks, or HTTP/CSP routes, before claiming UI work is complete or opening a PR, or when the user asks to test or preview the UI locally in a browser.
compatibility: Works across Claude Code, OpenAI Codex, Oh-my-pi (omp), OpenCode, and Antigravity (agy).
---

# Local UI Verify & Browser Preview (`local-ui-verify`)

Verify local web UI rendering, interactive state transitions, API contracts, and security headers automatically via headless Playwright Chromium, or start a persistent local preview server when the human partner wants to inspect the UI in their own browser.

## Modes of Operation

### 1. Default Mode: Automated Agent UI Verification (`--mode verify`)

Run this by default whenever you modify UI code, styles, or HTTP/content routes, or before claiming frontend work is ready:

```bash
python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py \
  --mode verify \
  --root . \
  --artifact-dir /tmp/local-ui-verify-artifacts \
  --json
```

*(If running from a repository where the skill is only installed globally, invoke `~/.agents/skills/local-ui-verify/scripts/local_ui_verify.py`.)*

What `--mode verify` checks automatically:
- Builds `web/out` (if missing or `--rebuild` is passed) and `cmd/blog-server` into an isolated temporary runtime on a free local port (or probes `--base-url <url>` for non-blog web apps). The isolated server is seeded from the test fixture `tests/fixtures/content/` (not the published `content/`), so custom-block contracts stay verifiable even when the blog has no posts.
- **HTTP & Security Header Contract**:
  - `GET /healthz` returns `200 OK` (`{"status":"ok"}`).
  - `GET /api/posts` and `GET /api/posts/{slug}` return `200 OK` with valid JSON payloads.
  - `GET /api/competencies` (removed) returns `404 Not Found`.
  - `GET /content/demos/*` returns `Content-Security-Policy: default-src 'self' 'unsafe-inline'; script-src 'self' 'unsafe-inline'; connect-src 'none'; frame-ancestors 'self'` and `X-Content-Type-Options: nosniff`.
  - Unmatched `/api/*` routes return `404 Not Found` (never SPA `index.html`).
- **Headless Browser (Playwright Chromium) Contract**:
  - Renders `/` on desktop (`1440x900`) and mobile (`390x844`), verifying the monochrome stone canvas (`rgb(250, 250, 249)`), the `Make Better World` hero, the Topics rail and Featured shelf, the absence of the Competency Matrix and `K3s Single-Pod` badge, and post cards.
  - Renders the fixture article `/post/?slug=cilium-zero-plaintext-k3s`, verifying custom blocks (`scrollHero`, `blogPicture`, `benchmarkChart`, `benchmarkTable`, `BlogV2MiniNav`, and `blogIframeTabs` poster guard).
  - Clicks `"Launch Interactive Demo"` and verifies that `<iframe sandbox="allow-scripts">` mounts cleanly.
  - Fails if any `console.error`, uncaught `pageerror`, or HTTP `>= 400` response occurs.
  - Saves full-page screenshots (`home-desktop.png`, `home-mobile.png`, `article-desktop.png`, `article-interactive-demo.png`) to `--artifact-dir`.

### 2. Manual Browser Mode: Interactive Human Preview (`--mode manual`)

When the user asks to test or preview the UI manually in their own browser:

```bash
# Start (or reuse) a persistent local preview server on http://localhost:8080
python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py \
  --mode manual \
  --root . \
  --port 8080
```

Then share the live URLs with the user (manual mode serves the real published `content/`):
- **Home**: `http://localhost:8080/`
- **Article Reader**: `http://localhost:8080/post/?slug=<post-slug>` (for any post in `content/posts/`)

Check status or stop the manual server when finished:

```bash
python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py --mode status --port 8080
python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py --mode stop --port 8080
```

### 3. Synchronize Multi-Agent Harnesses (`--install-harnesses`)

To install or refresh this skill across all 5 AI coding agents (`Claude Code`, `Codex`, `omp`, `OpenCode`, `Antigravity`) in `~/`:

```bash
python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py --install-harnesses
```
