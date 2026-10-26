# nacfson_blog Agent Guide (`proj-blog`)

## 1. Project Context

`nacfson_blog` is the cloud-native engineering blog and interactive CNCF portfolio hosted at `https://blog.nacfson.cloud` (`proj-blog`) on the `nacfson_pipeline` K3s platform.

Core architectural pillars:
- **Zero-Dependency Go 1.22 Stdlib Server (`cmd/blog-server/`, `internal/`)**: Serves the REST API (`/api/posts`, `/api/posts/{slug}`), static assets (`/content/assets/*`), strictly sandboxed interactive HTML demos (`/content/demos/*` with `default-src 'none'` CSP and `X-Content-Type-Options: nosniff`), and the Next.js static export (`web/out`).
- **Dual-Track GitOps Content Hot-Reload (`internal/syncer/flux_tarball.go`)**: Polls the in-cluster FluxCD `source-controller` `GitRepository/nacfson-blog-content` tarball via `ETag`/`If-None-Match` (`304 Not Modified`), enforcing zip-slip/symlink rejection and atomic `a`/`b` workspace swaps without restarting pods or rebuilding container images on `content/**` updates.
- **Monochrome Editorial Reader & Custom Block Dispatcher (`web/`)**: Next.js `14.2.15` (`output: 'export'`) UI (Tailwind `stone` palette: `#fafaf9` canvas, `#ffffff` cards, `#0c0a09` ink, serif headlines, no accent hue; posts carry a `topic` of `cloud-native` | `zero-trust` | `ai-agents` | `open-source`) with Kimi-style fenced Markdown blocks (`scrollHero`, `blogPicture`, `blogIframeTabs` with click-or-center poster guard and `<iframe sandbox="allow-scripts">`, `benchmarkChart`, `benchmarkTable`, `BlogV2MiniNav`, and isolated error-boundary fallback).

---

## 2. Multi-Agent Skills (`Claude Code`, `Codex`, `omp`, `OpenCode`, `Antigravity`)

This repository and local workstation provide unified skills across all 5 agent harnesses:

| Skill | Canonical Path | Purpose |
| :--- | :--- | :--- |
| [`local-ui-verify`](.agents/skills/local-ui-verify/SKILL.md) | [`.agents/skills/local-ui-verify/SKILL.md`](.agents/skills/local-ui-verify/SKILL.md) | **Default (`--mode verify`)**: Builds and boots an isolated local server, verifies HTTP/API/CSP contracts, and runs headless Playwright Chromium DOM/interaction/console-error checks + screenshots. **Manual (`--mode manual`)**: Starts a persistent local server on `http://localhost:8080` so the user can test the UI in their own browser. |

Cross-agent discovery entrypoints:
- **Antigravity (`agy`)**: [`.agents/skills/local-ui-verify/SKILL.md`](.agents/skills/local-ui-verify/SKILL.md), [`GEMINI.md`](GEMINI.md)
- **Claude Code (`claude`)**: [`.claude/skills/local-ui-verify/SKILL.md`](.claude/skills/local-ui-verify/SKILL.md), [`.claude/commands/local-ui-verify.md`](.claude/commands/local-ui-verify.md), [`CLAUDE.md`](CLAUDE.md)
- **OpenAI Codex (`codex`)**: [`.agents/skills/local-ui-verify/SKILL.md`](.agents/skills/local-ui-verify/SKILL.md), [`.codex/skills/local-ui-verify/SKILL.md`](.codex/skills/local-ui-verify/SKILL.md), [`AGENTS.md`](AGENTS.md)
- **Oh-my-pi (`omp`)**: [`.omp/skills/local-ui-verify/SKILL.md`](.omp/skills/local-ui-verify/SKILL.md), [`.omp/commands/ui.verify.md`](.omp/commands/ui.verify.md), [`AGENTS.md`](AGENTS.md)
- **OpenCode (`opencode`)**: [`.opencode/skills/local-ui-verify/SKILL.md`](.opencode/skills/local-ui-verify/SKILL.md), [`.opencode/commands/local-ui-verify.md`](.opencode/commands/local-ui-verify.md), [`AGENTS.md`](AGENTS.md)

---

## 3. Verification & Quality Gates

Run all verification suites before claiming work is complete or opening a pull request:

```bash
# 1. Go unit, race, and dual-track CI contract tests
go test -v -race ./... && go vet ./...

# 2. Frontend Vitest unit & component tests + static export build
(cd web && npm test && npm run build)

# 3. Cross-agent skill & automated headless Playwright UI verification
python3 -m unittest discover -s tests/skills
python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py --mode verify
```
