# Homepage ASCII Hero — Design

- **Status:** Approved for implementation. The author writes the tests first, then the code.
- **Date:** 2026-11-15
- **Builds on:** [`2026-11-14-3d-interactive-blog-intent.md`](2026-11-14-3d-interactive-blog-intent.md)
- **Reference implementation:** [`assets/ascii-hero-reference.html`](assets/ascii-hero-reference.html). Open it in a browser. All rendering behaviour (passes, glyph choice, lighting, motion, every number) comes from its `<script id="engine">` block. Port that; this doc does not repeat it.

## Decisions

| | |
|---|---|
| Where | Homepage only. A full-width hero band, nothing in posts. |
| Look | Torus knot drawn in monospace glyphs, Long ramp glyph set, **Ink** band (`#0c0a09`) with light stone glyphs |
| Engine | Plain WebGL 1. **No new dependency** (three.js rejected: about 150 KB gzipped for one hero). |
| Budget | ≤ 8 KB gzipped added to the homepage's first-load JS. Aim for ≥ 55 fps at 1440 px. |

## Anti-goals (any one of these fails the feature)

- Scroll pinning or hijacking, or anything that delays reaching the article grid.
- **Any console error** on the homepage, including without WebGL. `local-ui-verify` fails on console errors.
- A loading screen, spinner or layout shift.
- An accent hue. Only the stone palette is allowed.

## Placement (`web/src/app/page.tsx`)

- **New band.** It sits between the sub-navigation and the existing rail and grid, which stay unchanged.
  - At 760 px and wider: band height `clamp(420px, 60vh, 600px)`, copy at top-left laid over the canvas, knot on the right.
  - Below 760 px: the copy comes first, then a 320 px knot strip under it, so long copy never overlaps the knot. The strip stays hidden without WebGL.
- **The band's `<h1>` stays the only `h1`**, with text exactly `Make Better World`. `page.test.tsx` and `playwright_probe.mjs` both check it.
  - Remove the old `h1` and intro paragraph from the main column.
  - Move the intro paragraph into the band as the lede.
- **Link.** Add "Browse articles ↓" pointing to a new `id="articles"` on the grid wrapper.
- **Canvas.** `aria-hidden="true"`, `touch-action: pan-y`. The copy sits above it.
- **Colours.** Band `stone-950`, `h1` `stone-50`, lede `stone-400`, caption `stone-500`. Use the site's mono stack, not a web font; measure the cell width with `measureText`.
- **Leave out of the prototype:** the badges, the corner readout and the "Replay intro" button.

## Code layout

- **Pure logic, unit-testable in jsdom.** Put it under `web/src/lib/ascii/`:
  - `constants.ts`: every number and glyph string. The shaders take their numbers from here.
  - `matrix.ts`
  - `torusKnot.ts`
  - `glyphs.ts`: the ramp and edge-glyph choice, mirrored in TS for tests.
  - `layout.ts`: knot placement and grid size.
  - `shaders.ts`
- **WebGL calls in one place.** `renderer.ts` exports `createAsciiRenderer(canvas)`, which returns `{resize, frame, dispose}` or **`null`** on any failure. It never throws and never logs errors.
- **The component.** `web/src/components/AsciiHero.tsx` is `'use client'`. It renders the copy statically. After first paint (`requestIdleCallback`) it adds the canvas and owns:
  - the rAF loop
  - the observers
  - pointer input
  - reduced motion
  - cleanup

## Lifecycle and fallbacks

| Case | Behaviour |
|---|---|
| Hero off screen or tab hidden | No GL work. The clock is paused. |
| Resize or DPR change | Rebuild the canvas size, the glyph atlas and the off-screen targets. |
| WebGL context lost | `preventDefault()` and pause. Rebuild on restore. If the rebuild fails, use the no-WebGL fallback. |
| No WebGL, or a shader or framebuffer failure | Remove the canvas. The band shows its copy on ink. No message, no error log. |
| `prefers-reduced-motion` | One still frame, with no intro, sweep or twinkle. Redraw only on pointer move or resize. |
| Touch | The lens shows only while a finger is down. Horizontal drag turns the knot; vertical swipes scroll the page. |
| Unmount | Cancel rAF, disconnect observers, delete all GL objects, remove the canvas. |

## Tests to write first

- **Unit tests**
  - `torusKnot(1.1, .34, 300, 40, 2, 3)`:
    - has 12,341 vertices and 72,000 indices, and every index is in range
    - every normal has unit length
    - every vertex has radius ≤ 2.05
  - `matrix`:
    - identity
    - `rotateX(π/2)` maps (0, 1, 0) to (0, 0, 1)
    - `perspective` maps near to NDC −1 and far to +1
  - `glyphs`:
    - a covered cell is never a space
    - the ramp is monotonic
    - edge glyphs are `|`, `\`, `-`, `/` at 0, π/4, π/2 and 3π/4
  - `layout`:
    - the wide/narrow switch happens at 760 px
    - the knot stays inside the frame at 1440 and 390 px
  - `shaders`: the GLSL contains the numbers from `constants.ts`.
- **Component tests (jsdom)**
  - With `getContext → null`:
    - the `h1`, lede and link render
    - no canvas remains
    - `console.error` is never called
  - With `renderer` mocked:
    - mount schedules frames
    - reduced motion draws exactly one frame
    - unmount calls `dispose` and cancels rAF
- **Existing tests:** `page.test.tsx`, plus all of CLAUDE.md §3 including `local_ui_verify.py --mode verify`.
- **Manual checks**
  - Looks like the reference at 1440 and 390 px.
  - Reduced motion is emulated correctly.
  - Run with `--disable-webgl` and confirm the fallback.
  - The Performance panel shows no GPU work when the band is scrolled away.
  - Lens and swipe work on a real phone.
  - `next build` shows ≤ 8 KB added.
