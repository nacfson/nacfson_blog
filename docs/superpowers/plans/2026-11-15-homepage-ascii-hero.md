# Homepage ASCII Hero Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the homepage-only ASCII knot hero band (long ramp, ink tone) using plain WebGL 1 with no new dependencies.

**Architecture:**
- Pure TypeScript modules under `web/src/lib/ascii/` hold the math, geometry, glyph rules, layout and GLSL. All of them are unit-tested in jsdom.
- A single `renderer.ts` owns every WebGL call and returns `null` on any failure.
- `AsciiHero.tsx` renders static copy, then mounts the canvas after first paint and owns the frame loop.
- `page.tsx` places the band above the existing grid.

**Tech Stack:** Next.js 14 static export, React 18, TypeScript, Tailwind stone palette, Vitest + Testing Library (jsdom), WebGL 1.

**Spec:** `docs/superpowers/specs/2026-11-15-homepage-ascii-hero-design.md`. Reference code: `docs/superpowers/specs/assets/ascii-hero-reference.html`, in its `<script id="engine">` block. Every number not listed here comes from that block.

## Global Constraints

- No new entries in `web/package.json` `dependencies`.
- ≤ 8 KB gzipped added to the homepage's first-load JS.
- No `console.error` from hero code in any environment, including jsdom and browsers without WebGL.
- The first and only `h1` on the homepage is exactly `Make Better World`.
- Stone palette only: band `#0c0a09`, bright ink `#fafaf9`, dim ink `#57534e`, dust `#292524`.
- DPR cap 1.5. No scroll pinning or hijacking.

## Review Focus

1. **jsdom or browsers without `WebGLRenderingContext`.** The renderer must return `null` before it calls `getContext`; jsdom logs "Not implemented" as a console error otherwise. Pinned in Task 3.
2. **The component unmounts before its idle callback fires.** No renderer should be created and no rAF should leak. Pinned in Task 4.
3. **Reduced motion.** Exactly one frame is drawn, then nothing more until a pointer move or resize. Pinned in Task 4.
4. **A zero-size band** (hidden or not yet laid out). `resize(0, 0)` must not create zero-size framebuffers. Pinned in Task 2, where `gridSize` clamps to at least 1.
5. **A topic filter or empty state** must still render under the band. Covered by the existing `page.test.tsx` tests, which must stay green (Task 5).

---

### Task 1: Math and geometry

**Files:**
- Create: `web/src/lib/ascii/constants.ts`, `web/src/lib/ascii/matrix.ts`, `web/src/lib/ascii/torusKnot.ts`
- Test: `web/src/lib/ascii/__tests__/matrix.test.ts`, `web/src/lib/ascii/__tests__/torusKnot.test.ts`

**Interfaces:**
- **Produces:** `ASCII`, a constants object. It has these fields:
  - `cellPx: 13`, `cellHeightRatio: 1.12`, `lensPx: 95`, `dprCap: 1.5`, `supersample: 2`
  - `ramp`, `edges: '|/-\\'`
  - `camera: {distance: 7, fovDeg: 32, near: 0.1, far: 50}`
  - `knot: {R: 1.1, tube: 0.34, tubular: 300, radial: 40, p: 2, q: 3, radius: 2.0}`
  - `lum: {gamma: 1.15}`
  - `edge: {depth: 0.09, normal: 1.35}`
  - `intro: {ms: 1400}`
  - `sweep: {periodS: 7, halfWidth: 0.07, boost: 0.3}`
  - `stillT: 3.2`
  - `ink: {bright: [250,250,249], dim: [87,83,78], dust: [41,37,36]}`
  - `wideMinPx: 760`
- **Produces:** `type Mat4 = Float32Array`; `identity()`, `multiply(a, b)`, `rotateX(rad)`, `rotateY(rad)`, `translate(x, y, z)`, `scale(s)`, `perspective(fovyRad, aspect, near, far)`, `normalMatrix3(m: Mat4): Float32Array`. All column-major.
- **Produces:** `torusKnot(R, tube, tubular, radial, p, q): { positions: Float32Array; normals: Float32Array; indices: Uint16Array }`

- [ ] **Step 1: Write failing tests**
  - `multiply(identity(), m)` equals `m`.
  - `rotateX(Math.PI/2)` applied to (0, 1, 0, 0) gives (0, 0, 1, 0) within 1e-6.
  - `perspective(1, 1.5, 0.1, 50)` maps z = −0.1 to NDC −1 and z = −50 to NDC +1. Divide by w; check within 1e-5.
  - `normalMatrix3(multiply(scale(2), rotateY(0.5)))` columns, normalized, equal `rotateY(0.5)`'s upper 3×3.
  - `torusKnot(1.1, .34, 300, 40, 2, 3)`:
    - `positions.length/3 === 12341`
    - `indices.length === 72000`
    - `max(indices) < 12341`
    - every normal has length within 1 ± 1e-6
    - every vertex has radius ≤ 2.05
- [ ] **Step 2:** Run `cd web && npx vitest run src/lib/ascii`. Expected: FAIL, modules not found.
- [ ] **Step 3:** Implement the three files, porting `M` and `torusKnot` from the reference.
- [ ] **Step 4:** Run the same command. Expected: PASS.
- [ ] **Step 5:** Commit `feat(hero): add ascii math, knot geometry and constants`.

### Task 2: Glyph rules, layout and shaders

**Files:**
- Create: `web/src/lib/ascii/glyphs.ts`, `web/src/lib/ascii/layout.ts`, `web/src/lib/ascii/shaders.ts`
- Test: `web/src/lib/ascii/__tests__/glyphs.test.ts`, `layout.test.ts`, `shaders.test.ts`

**Interfaces:**
- **Consumes:** `ASCII` from Task 1.
- **Produces:**
  - `rampIndex(lum: number, coverage: number): number`
  - `edgeGlyph(angleRad: number): '|' | '/' | '-' | '\\'`
  - `fitKnot(cssW: number, cssH: number): { scale: number; x: number; y: number; wide: boolean }`
  - `gridSize(w: number, h: number, cw: number, ch: number): { cols: number; rows: number }`
  - `SCENE_VS`, `SCENE_FS`, `QUAD_VS`, `ASCII_FS: string`

- [ ] **Step 1: Write failing tests**
  - `rampIndex`:
    - `rampIndex(0, 1) === 1`
    - `rampIndex(0.5, 0) === 0`
    - `rampIndex(1, 1) === N-1`, where N is `[...ASCII.ramp].length`
    - non-decreasing over lum 0..1 in steps of 0.01
  - `edgeGlyph`:
    - `edgeGlyph(0) === '|'`, `edgeGlyph(Math.PI/4) === '\\'`, `edgeGlyph(Math.PI/2) === '-'`, `edgeGlyph(3*Math.PI/4) === '/'`
    - `edgeGlyph(a + Math.PI) === edgeGlyph(a)`
  - `fitKnot`:
    - `fitKnot(759, 600).wide === false`
    - `fitKnot(760, 600).wide === true`
    - for 1440×600 and 390×600, `|x| + knot.radius*scale ≤ visibleHalfWidth` and `|y| + knot.radius*scale ≤ visibleHalfHeight`
  - `gridSize(0, 0, 8, 14)` gives `{cols: 1, rows: 1}`.
  - `ASCII_FS` contains `ASCII.edge.depth.toFixed(2)` and `ASCII.lum.gamma.toFixed(2)`; `SCENE_FS` contains `40.0`.
- [ ] **Step 2:** Run the tests. Expected: FAIL.
- [ ] **Step 3:** Implement.
  - Port the GLSL from the reference. Interpolate constants with a helper `f(n) => n.toFixed(2)` so values stay valid GLSL floats.
  - `rampIndex` mirrors the shader: `max(cov > 0.5 ? 1 : 0, floor(clamp(lum*cov, 0, 0.999)*N))`.
- [ ] **Step 4:** Run the tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat(hero): add glyph rules, layout and shaders`.

### Task 3: WebGL renderer

**Files:**
- Create: `web/src/lib/ascii/renderer.ts`
- Test: `web/src/lib/ascii/__tests__/renderer.test.ts`

**Interfaces:**
- **Consumes:** everything from Tasks 1–2.
- **Produces:** `createAsciiRenderer(canvas: HTMLCanvasElement): AsciiRenderer | null`, with this shape:

  ```ts
  type AsciiRenderer = {
    resize(cssW: number, cssH: number, dpr: number): void
    frame(s: FrameState): void
    dispose(): void
  }

  type FrameState = {
    t: number
    intro: number        // 0..1
    sweep: number        // -1 = off
    yaw: number
    tiltX: number
    tiltY: number
    pointer: [number, number] | null   // CSS px from the canvas top-left
  }
  ```

- [ ] **Step 1: Write failing tests**
  - With `WebGLRenderingContext` undefined (jsdom default), it returns `null` and `canvas.getContext` is never called (spy).
  - With `WebGLRenderingContext` stubbed but `getContext` returning `null`, it returns `null`.
  - `console.error` is never called in either case.
- [ ] **Step 2:** Run the tests. Expected: FAIL.
- [ ] **Step 3:** Implement.
  - Port `setup`, `buildAtlas`, `drawKnot` and `render` from the reference.
  - The atlas font is the Tailwind mono stack `ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace`, weight 500. Cell width is `measureText('M').width` rounded, with a minimum of 4.
  - Wrap creation in try/catch and return `null` on any failure.
  - `dispose` deletes all buffers, textures, framebuffers, renderbuffers and programs.
- [ ] **Step 4:** Run the tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat(hero): add plain WebGL ascii renderer`.

### Task 4: `AsciiHero` component

**Files:**
- Create: `web/src/components/AsciiHero.tsx`
- Test: `web/src/components/__tests__/AsciiHero.test.tsx`

**Interfaces:**
- **Consumes:** `createAsciiRenderer` from Task 3, which is mocked in the lifecycle tests with `vi.mock('../../lib/ascii/renderer')`.
- **Produces:** `export default function AsciiHero(): JSX.Element`. It renders:

  ```
  <section aria-labelledby="hero-title">
    <h1 id="hero-title">Make Better World</h1>
    <p>  intro lede (today's homepage paragraph)
    <a href="#articles">Browse articles ↓</a>
  </section>
  ```

- [ ] **Step 1: Write failing tests**
  - **Fallback** (no mock, jsdom): render it, then flush timers.
    - The heading "Make Better World", the lede and the link "Browse articles ↓" (href `#articles`) are present.
    - The container has no `canvas`.
    - `console.error` was not called.
  - **Lifecycle** (mocked renderer returns a fake `{resize, frame, dispose}`): with fake timers and a stubbed `requestAnimationFrame` queue, plus stubbed `IntersectionObserver` and `ResizeObserver` that report visible:
    - `frame` is called after an idle tick and one rAF.
    - Unmount calls `dispose` and `cancelAnimationFrame`.
  - **Reduced motion** (`matchMedia` returns `matches: true` for `(prefers-reduced-motion: reduce)`): `frame` is called exactly once across three rAF ticks.
  - **Early unmount:** unmount before the idle tick; `createAsciiRenderer` is never called.
- [ ] **Step 2:** Run the tests. Expected: FAIL.
- [ ] **Step 3:** Implement.
  - Port the loop, pointer, drag, damping and visibility logic from the reference.
  - Band classes: `relative overflow-hidden bg-stone-950 h-[600px] md:h-[clamp(420px,60vh,600px)]`.
  - Copy block z-indexed above the canvas, with `pointer-events-none` except on the link.
  - Canvas `absolute inset-0 h-full w-full`, `aria-hidden`, `touch-action: pan-y` on the section.
- [ ] **Step 4:** Run the tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat(hero): add AsciiHero component`.

### Task 5: Homepage integration

**Files:**
- Modify: `web/src/app/page.tsx`
- Test: `web/src/app/__tests__/page.test.tsx`

**Interfaces:**
- **Consumes:** `AsciiHero` from Task 4.

- [ ] **Step 1: Write a failing test** in `page.test.tsx`.
  - Exactly one `h1` is present (`getAllByRole('heading', {level: 1})` has length 1).
  - The link "Browse articles ↓" has href `#articles`.
  - `document.getElementById('articles')` contains the Topics navigation.
- [ ] **Step 2:** Run it. Expected: FAIL.
- [ ] **Step 3:** Implement.
  - `HomePage` returns `<><AsciiHero /><div id="articles" …existing grid…></>`.
  - Remove the old `<section>` holding the `h1` and paragraph from the main column.
- [ ] **Step 4:** Run `cd web && npm test`. Expected: all tests PASS, including the existing ones.
- [ ] **Step 5:** Commit `feat(home): place ascii hero band above the article index`.

### Task 6: Verification

- [ ] Run `go test -race ./... && go vet ./...`. Expected: PASS.
- [ ] Run `cd web && npm test && npm run build`. Expected: PASS. Compare the build's `/` first-load JS with `main` and confirm the growth is ≤ 8 KB.
- [ ] Run `python3 -m unittest discover -s tests/skills && python3 .agents/skills/local-ui-verify/scripts/local_ui_verify.py --mode verify`. Expected: PASS with 0 console errors. Look at the homepage screenshots.
- [ ] Commit any fixes. Push only when the author asks.
