# 3D Interactive Blog — Intent

- **Status:** Intent captured; design not started (placement still open, see below)
- **Date:** 2026-11-14

## Intent

Bring 3D, scroll-driven interactive experiences to `blog.nacfson.cloud` —
in the spirit of Kimi Code's 3D websites — **without hurting the reading
UI/UX** of the monochrome editorial reader.

### References

- Video: "Build 3D Websites with Kimi Code – Tutorial #4" — https://www.youtube.com/watch?v=GZFoexLsMi8
- Site: https://4osiyyp5h7mwu.ok.kimi.link ("Cheetah / Run — Built to Move")

### What the reference site does (from its source)

- Vanilla three.js (import map, no React); one fixed full-viewport canvas, minimal DOM text.
- Scroll progress (damped) drives a camera rig along a road while a line-rendered,
  morph-animated cheetah runs ahead.
- 3D UI widgets (card stack, flow nodes, glass play button, dial, waveform) placed
  along the path; they rise into view at an entrance threshold and are raycast-clickable.
- ASCII "destination" finale, HUD overlay, loading screen with progress, color presets
  saved to `localStorage`.
- Good hygiene: honours `prefers-reduced-motion`, DPR capped at 1.5, pauses on `document.hidden`.
- Weak spots for a blog: auto-scrolls to the end at 98% progress (scroll hijack), and a
  heavy vertex payload (tens of MB, estimated from buffer offsets) behind a loader.

## Goals

- Borrow: scroll-linked damped camera, entrance reveals, line/wireframe rendering,
  sparse click interactions, DPR cap + offscreen/hidden pause.
- Explain the author's own systems in 3D (e.g. GitOps hot-reload: commit → Flux
  source-controller → tarball → `a`/`b` workspace swap; K3s scheduling; zero-trust boundaries).
- Stay inside the existing visual language: Tailwind `stone` palette, no accent hue.

## Non-goals / anti-goals

- No full-page canvas over article prose; prose must remain readable without 3D.
- No scroll hijacking or auto-snap; native page scroll always wins.
- No multi-MB model download blocking first paint; no loading screens in front of articles.
- No accent colours (the reference's blue `#2260d9` does not carry over).

## UX guardrails

- Poster-first; mount 3D only on click or when centred (existing `click-or-center` guard).
- `prefers-reduced-motion` or no WebGL → static poster.
- Pause when offscreen or tab hidden; DPR ≤ 1.5; frame-time budget.
- Every scene has a text caption so the post works without it.

## Constraints found in the codebase

- `/content/demos/*` CSP (`internal/server/router.go`) is `connect-src 'none'` and
  self-only scripts: demos can't `fetch` models (GLTFLoader fails) or load three.js from
  a CDN. Models must be inlined/vendored, or CSP widened to `connect-src 'self'`.
- Demos render in `<iframe sandbox="allow-scripts">` via `blogIframeTabs`.
- `web/` currently has only `next`, `react`, `react-dom` as runtime deps.

## Candidate placements (not yet chosen)

1. **Full experience behind a poster in a post** — standalone HTML in `content/demos/`,
   opened via `blogIframeTabs` (fullscreen supported). No reader changes. *(Recommended first step.)*
2. **3D cover at the top of a post** — ~1–1.5 screens of pinned scroll scene, then hand off
   to normal prose, with a "Skip to article" link. Likely a new native block.
3. **Homepage front door** — full scene introducing the author, post index below.
   Biggest impression, furthest from the editorial tone.

## Open question

Where should readers meet this experience: the homepage, the top of each post, or as a
special piece inside selected posts? This decides whether the work is a new homepage or
a new post block.
