# Blog Studio: Local Git-Backed Writing Platform

Date: 2026-11-04 · Status: approved in chat ("local-only. Writing, save, publish … replaced by git
commands" / "Straight to that branch" / mockup: "It looks good. Create spec for it.")

Mockup (source of truth for layout and interaction):
[`docs/superpowers/mockups/2026-11-04-blog-studio.html`](../mockups/2026-11-04-blog-studio.html)

## 1. Intent

A local-only writing app for `nacfson_blog`. The author writes in a browser tab on their own
machine; **Save** and **Publish** are implemented as git operations, so the author never types git
commands. Publishing = pushing a content-only commit to the Flux-tracked branch, which
`GitRepository/nacfson-blog-content` picks up and the running `blog-web` pod hot-reloads in ~30s
(Track B: no image build, no pod restart).

## 2. Decisions

### User
- Local-only. Never deployed, never part of the production image.
- Write / Save / Publish are replaced by git operations executed by the studio.
- Publish pushes **directly** to the Flux-tracked branch (no PR).
- The mockup's layout and flow are approved.

### Mine (reviewable)
- Each draft lives on its own local branch `studio/<slug>` in its own worktree
  `.worktrees/studio-<slug>/` (`.worktrees/` is already gitignored). The author's main checkout is
  never read from or written to by Save/Publish, so unrelated uncommitted work cannot leak.
- Publish squashes the draft history into one commit `content: publish <slug>`; draft commits never
  reach the remote.
- Validation uses the production parser (`internal/content`), not a re-implementation.
- The preview uses the production renderer (`web/src/components/MarkdownDispatcher.tsx`), imported
  directly.
- Publish branch defaults to `main`, configurable with `--publish-branch`.

## 3. Git model

| Action | Git effect | Leaves machine? |
| :--- | :--- | :--- |
| New draft | `git fetch origin <pb>` → `git worktree add -b studio/<slug> .worktrees/studio-<slug> origin/<pb>` | No |
| Edit published post | Same as New draft, branched from `origin/<pb>` where the post file already exists | No |
| Write (autosave) | Atomic write of the post file inside the draft worktree (temp file + rename). No git. | No |
| Save | Validate → `git add <post file> [assets…]` → `git commit -m "draft(<slug>): save <HH:MM:SS>"` | No |
| Publish | See §3.2 | **Yes** |
| Discard draft | `git worktree remove --force .worktrees/studio-<slug>` → `git branch -D studio/<slug>` (UI confirm) | No |

`<pb>` = publish branch. All commands run via `exec.Command("git", args...)` (no shell), using the
author's own git config, identity, and SSH credentials.

### 3.1 Slug renames and date changes
The worktree/branch name is fixed at creation (`studio/<initial-slug>`). If the author changes the
slug or date, the post file path changes; Save runs `git mv <old> <new>` so history follows and the
publish commit is a rename, not add + orphan. The branch is not renamed.

### 3.2 Publish algorithm
A single publish worktree `.worktrees/studio-publish/` (detached) is reused.

1. If the draft has unsaved changes: run Save (abort publish if Save fails).
2. `git fetch origin <pb>`
3. `git -C .worktrees/studio-publish checkout --detach origin/<pb>` then `reset --hard origin/<pb>`
4. `git -C .worktrees/studio-publish merge --squash studio/<slug>`
5. `git -C .worktrees/studio-publish diff --cached --name-only` → **abort** if any path is outside
   `content/`.
6. Validate the **whole** `content/posts/` of the publish worktree with the production parser
   (catches duplicate slugs and any other post that would break `FolderStore.Reload()`).
7. `git -C .worktrees/studio-publish commit -m "content: publish <slug>"`
8. `git -C .worktrees/studio-publish push origin HEAD:<pb>`
9. On non-fast-forward rejection: repeat steps 2–8 **once**, then surface the error. Never
   `--force`.
10. On success: remove the draft worktree and delete `studio/<slug>`.

Any failure before step 8 leaves the remote untouched and the draft branch intact. If the squash in
step 4 conflicts (someone else edited the same post on `<pb>`), abort with
`git merge --abort`/`reset --hard` and report the conflicting paths; v1 does not resolve conflicts.

### 3.3 Startup reconciliation
On boot the studio lists `git worktree list --porcelain` and `git branch --list 'studio/*'`,
re-attaching existing drafts and pruning worktrees whose branch is gone (`git worktree prune`).
`studio/<slug>` branches without a worktree get one re-created.

### 3.4 Derived draft status (shown in header + sidebar)
| Status | Rule | Header text |
| :--- | :--- | :--- |
| `unsaved` | worktree file differs from `HEAD` of `studio/<slug>` | "Unsaved changes · autosaved to worktree HH:MM:SS" |
| `new` | no commits beyond the branch point, post absent on `origin/<pb>` | "New draft · nothing committed yet" |
| `saved` | `git rev-list --count origin/<pb>..studio/<slug>` > 0 | "Saved locally · N commits not on <pb>" |
| `live` | post file on `origin/<pb>` and no draft branch (or draft identical) | "Published · in sync with origin/<pb>" |

Drafts list = everything not `live`; Published list = posts present on `origin/<pb>`, newest first. A
published post being edited appears in both.

### 3.5 Concurrency
All git operations are serialized through one mutex in the studio process. A lock file
`.worktrees/studio.lock` (PID) prevents two studio instances on the same repo.

## 4. Post file contract

Must round-trip through `internal/content.parsePost`. The studio serializes frontmatter itself
(form → file), in this fixed order:

```
---
title: "<title>"
slug: "<slug>"
date: "<YYYY-MM-DD>"
topic: "<topic>"
project: "<project>"      # omitted when empty
summary: "<summary>"
tags: ["a", "b"]
---
<body>
```

- Values are wrapped in double quotes **without escaping**: the Go parser only strips the outer
  quote pair, so inner `"` must stay raw. Newlines are rejected in all frontmatter fields.
- `readTimeMinutes` is not written (server computes 200 wpm); preserved if present in an existing post.
- Unknown keys in an existing post are preserved on load and re-emitted after the known keys.
- File path: `content/posts/<date>-<slug>.md`.
- Opening an existing post parses its frontmatter with the same Go parser (via the API), not in JS.

## 5. Validation and lint

`POST /studio/api/validate` returns `{issues: [{severity, message, field?, line?}]}`.

**Errors** (block Save and Publish): these exactly mirror `parseFrontmatter` + store rules, invoked through a
new exported `content.ValidatePost(data []byte, filename string) (*Post, error)` plus
`content.ValidateDir(root string) error` (whole-index check: duplicate slugs).
- title missing · slug missing / not `[A-Za-z0-9_-]+` / duplicate · date missing or not
  `YYYY-MM-DD` (studio-only stricter rule; the server just requires non-empty) · topic not one of
  `cloud-native | zero-trust | ai-agents | open-source`
- invalid JSON in any of the 5 custom block fences (`scrollHero`, `blogPicture`, `blogIframeTabs`,
  `benchmarkChart`, `benchmarkTable`), with line number

**Warnings** (never block), computed on the body with fenced blocks and inline code blanked out
(line positions preserved):
- `![…](…)`: "Markdown images don't render: use a blogPicture block"
- `^\d+\. `: "Ordered lists render as a plain paragraph: use "-" bullets"
- `^|…|$`: "Pipe tables don't render: use a benchmarkTable block"
- single `*x*` / `_x_`: "Italics aren't supported"
- raw HTML tags: "Raw HTML is shown as text"
- `^# `: "Body H1 duplicates the article title"
- `` **`x`** ``: "Code inside bold renders literal backticks" (renderer quirk seen in mockup)
- `/content/assets/…` or `/content/demos/…` reference that doesn't exist in the draft worktree
- empty summary

Warnings link to their line; clicking selects it in the editor.

## 6. Assets and demos

- `POST /studio/api/drafts/{slug}/assets` (multipart) writes into the draft worktree's
  `content/assets/` or `content/demos/`; staged on the next Save.
- Allowed: `.svg .png .jpg .jpeg .webp .gif` → assets; `.html` → demos. Max 10 MiB per file; reject if
  the resulting `content/` would exceed 50 MiB (syncer limits in `internal/syncer/flux_tarball.go`).
- Filenames normalized to `[a-z0-9._-]`; collisions get `-2`, `-3`.
- Demo lint (warnings): `fetch(`, `XMLHttpRequest`, `WebSocket`, `EventSource`, external
  `src=`/`href=` (`http(s)://`), `<form`, since all of these are blocked by the demo CSP / sandbox.
- Drag-and-drop an image onto the editor = upload + insert a `blogPicture` block with `src` filled and
  a placeholder `alt`.
- SVG uploads are served by the studio with the same headers as production; no sanitization in v1
  (single trusted author).

## 7. Architecture

```
cmd/blog-studio/main.go         flags, lock file, boot reconciliation, http.Server on 127.0.0.1
internal/studio/
  git.go                        Git runner (exec, no shell), typed wrappers, output capture for the log
  drafts.go                     draft lifecycle + status derivation (§3)
  publish.go                    publish algorithm (§3.2)
  serialize.go                  frontmatter round-trip (§4)
  lint.go                       warnings (§5) + demo lint (§6)
  assets.go                     upload handling (§6)
  api.go                        /studio/api/* handlers, localhost guard, SSE git log
  embed.go                      //go:embed studio/dist (built SPA)
internal/content/
  validate.go                   exported ValidatePost / ValidateDir (wrap existing unexported code)
studio/                         Vite + React 18.3.1 + TypeScript + Tailwind 3.4 SPA
  vite.config.ts                alias @web → ../web/src; resolve.dedupe react/react-dom
  tailwind.config.ts            content includes ../web/src/components/**/* (renderer classes)
  src/App.tsx                   layout from mockup
  src/components/*              Sidebar, FrontmatterForm, Editor, Preview, ChecksPanel, GitLog,
                                PublishDialog, InsertMenu
```

- **Renderer sharing:** `Preview` renders the article header like `web/src/app/post/page.tsx` and
  the body with `@web/components/MarkdownDispatcher`. `web/src/components` only depends on React and
  `web/src/lib/types`, so it imports cleanly into Vite. `web/` itself is not modified.
- **Editor:** CodeMirror 6 (`@codemirror/lang-markdown`, JSON highlighting inside block fences),
  preview updates debounced at 150 ms, autosave at 700 ms idle.
- **Preview assets:** the studio serves `/content/assets/*` and `/content/demos/*` from the active
  draft worktree, with the same CSP / `nosniff` headers as `internal/server/router.go`, so
  `blogPicture`, `scrollHero` and `blogIframeTabs` render with real files.
- **Build:** `make studio` (or `go generate`) runs `npm --prefix studio ci && npm --prefix studio run build`
  then `go build -o bin/blog-studio ./cmd/blog-studio`. `bin/` is gitignored.
- **Production isolation:** the Dockerfile builds only `./cmd/blog-server` and copies only `web/`, so
  neither `cmd/blog-studio`, `internal/studio`, nor `studio/` reach the image.

### 7.1 Flags
| Flag | Default | Meaning |
| :--- | :--- | :--- |
| `--repo` | current dir | Repository root (must contain `content/posts`) |
| `--addr` | `127.0.0.1:4321` | Listen address; non-loopback hosts are refused |
| `--publish-branch` | `main` | Branch Flux tracks |
| `--remote` | `origin` | Remote to fetch/push |
| `--open` | `true` | Open the browser on start |

## 8. HTTP API (JSON, localhost only)

| Method + path | Purpose |
| :--- | :--- |
| `GET /studio/api/state` | Drafts + published posts with derived status, publish branch, remote |
| `POST /studio/api/drafts` | `{title?}` → new draft (slug `untitled-N` until titled) |
| `POST /studio/api/drafts/from-post/{slug}` | Start editing a published post |
| `GET /studio/api/drafts/{slug}` | Frontmatter fields + body + issues |
| `PUT /studio/api/drafts/{slug}` | Autosave `{frontmatter, body}` (no commit) |
| `POST /studio/api/drafts/{slug}/save` | Validate + commit |
| `GET /studio/api/drafts/{slug}/publish-plan` | Checks, unified diff vs `origin/<pb>`, step list |
| `POST /studio/api/drafts/{slug}/publish` | Run §3.2; streams step progress |
| `DELETE /studio/api/drafts/{slug}` | Discard (requires `{"confirm": "<slug>"}`) |
| `POST /studio/api/drafts/{slug}/assets` | Upload asset/demo |
| `POST /studio/api/validate` | Validate unsaved `{frontmatter, body}` |
| `GET /studio/api/events` | SSE stream: git commands + output for the Git activity panel |

Guards: reject requests whose `Host` is not `127.0.0.1:<port>`/`localhost:<port>`; mutating
requests also require a matching `Origin` and `Content-Type: application/json` (or multipart for
uploads). Slugs in paths validated with the same rule as `isValidSlug`.

## 9. UI (per mockup)

- **Header:** brand + `127.0.0.1:4321 · local only`, current branch, status (§3.4),
  **Save** (Ctrl/⌘+S, disabled when clean), **Publish…** (disabled when live and clean).
- **Sidebar:** "+ New draft"; *Drafts*; *Published · origin/<pb>*. Each item: title, status dot
  (hollow = unsaved, grey = saved/new, black = live), label, date.
- **Frontmatter form:** Title (serif), Slug (`(auto)` while derived from title), Date, Topic select
  (4 values), Summary, Project, Tags (comma-separated). Fields with errors get the ink left-bar style.
- **Editor | Preview** split; editor header shows file path + insert buttons for the 5 blocks
  (templates exactly as in the mockup).
- **Checks** panel (`N errors · M warnings`, click to jump) | **Git activity** log (dark, commands
  prefixed `$`).
- **Publish dialog:** Checks list → Changes (`+A −D`, unified diff with 2 lines of context, new vs
  modified header) → "Will run" step list that ticks through live → footer
  "Pushes directly to **origin/<pb>**. Flux syncs the live site in ~30s." → *Cancel* / *Push to <pb>*
  (disabled with errors). On success: "Published `<sha>` … Live in ~30s at /post/?slug=<slug>",
  buttons become *Done* / *Pushed*.
- Monochrome stone palette, serif headlines, 4px radius, no accent hue (matches the blog).

## 10. Repository changes outside the studio

- `internal/content/validate.go`: exported `ValidatePost`, `ValidateDir` (additive; this is the only
  production-code change and triggers one image build when merged).
- `.github/workflows/publish-ghcr-image.yml`: add `!cmd/blog-studio/**` and
  `!internal/studio/**` after the existing includes (both `push` and `pull_request`), and add a
  separate lightweight `studio-ci` job (Go tests for `internal/studio`, `npm test` in `studio/`)
  triggered by those paths + `studio/**`.
- `.gitignore`: `studio/node_modules/`, `studio/dist/`, `bin/`.
- `AGENTS.md`: a short "Blog Studio" section (how to run, safety rules).

## 11. Testing

- **Go, `internal/studio`:** tests run against a temp bare repo as `origin` + a clone:
  new draft → save → publish happy path; non-content path in squash aborts; invalid post aborts
  before commit; duplicate slug on `<pb>` aborts; push rejected → one retry succeeds; second
  rejection surfaces error with remote unchanged; squash conflict aborts cleanly; slug rename uses
  `git mv`; startup reconciliation; status derivation table (§3.4); serialize round-trip through
  `content.ValidatePost` (quotes, unicode, tags, preserved unknown keys); API localhost/Origin guards.
- **Go, `internal/content`:** `ValidatePost` / `ValidateDir` table tests.
- **Studio (Vitest + Testing Library):** form → serialized frontmatter, slug auto/manual, lint
  click-to-line, publish dialog disabled on errors, status labels.
- **End-to-end (Playwright, local):** boot `blog-studio` against a temp clone with a bare origin,
  create a draft, save, publish, assert the commit on the bare repo's `main` touches only
  `content/posts/…` and that the screenshot matches the mockup layout. No console errors.
- Existing gates unchanged: `go test -race ./... && go vet ./...`, `(cd web && npm test && npm run build)`,
  `python3 -m unittest discover -s tests/skills`, `local-ui-verify --mode verify`.

## 12. Phases

1. **v1 (this spec):** §3–§11 except form editors for blocks.
2. **v2:** grid editor for `benchmarkTable`, series editor for `benchmarkChart`, Unpublish
   (`git revert` of the publish commit → push, with confirm), per-post history (`git log --follow`).
3. **Later:** optional AI helpers (summary, tags, alt text), on demand only.

## 13. Non-goals

- No auth, multi-user, or remote access; no deployment of the studio.
- No PR flow; no force-push; no conflict resolution UI.
- No changes to the production renderer or server behavior (lint steers authors instead).
- No dark mode.

## 14. Open items

- **Flux branch:** `docs/superpowers/plans/2026-10-24-cncf-blog-portfolio.md` specifies
  `GitRepository/nacfson-blog-content` with `ref.branch: main`, but the manifest is not yet in the local
  `nacfson_pipeline` checkout. `origin/main` and `origin/master` currently point to the same commit.
  Confirm before first real publish; `--publish-branch` covers either answer.
