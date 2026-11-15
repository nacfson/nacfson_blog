/**
 * Every tunable value of the homepage ASCII hero. Shaders read their numbers
 * from here (see shaders.ts) so tests and GLSL cannot drift apart.
 * Values come from docs/superpowers/specs/assets/ascii-hero-reference.html.
 */
export const ASCII = {
  /** Glyph size in CSS px; cell height = cellPx * cellHeightRatio. */
  cellPx: 13,
  cellHeightRatio: 1.12,
  /** Radius of the pointer lens in CSS px; cells inside are half size. */
  lensPx: 95,
  dprCap: 1.5,
  /** Off-screen passes render at this multiple of the cell grid. */
  supersample: 2,
  /** Long ramp, from empty to densest. */
  ramp: " .'`^\",:;Il!i><~+_-?][}{1)(|tfjrxnuvczXYUJCLQ0OZmwqpdbkhao*#MW&8%B@$",
  /** Outline glyphs, in shader index order. */
  edges: '|/-\\',
  font: '500 {px}px ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
  camera: { distance: 7, fovDeg: 32, near: 0.1, far: 50 },
  knot: { R: 1.1, tube: 0.34, tubular: 300, radial: 40, p: 2, q: 3, radius: 2.0 },
  /** View-space directional lights: diffuse + Blinn-Phong specular. */
  lights: [
    { dir: [3, 4, 5], k: 0.85 },
    { dir: [-5, 2, -3], k: 0.7 },
    { dir: [-2, -3, 4], k: 0.12 },
  ],
  light: { ambient: 0.04, specPower: 40, specWeight: 0.45 },
  lum: { gamma: 1.15 },
  edge: { depth: 0.09, normal: 1.35 },
  intro: { ms: 1400 },
  sweep: { periodS: 7, halfWidth: 0.07, boost: 0.3 },
  motion: { rotX: 0.21, rotY: 0.33, tiltX: 0.22, tiltY: 0.3, dragRadPerWidth: 4, damping: 6 },
  /** Time used for the single frame drawn under reduced motion. */
  stillT: 3.2,
  ink: { bright: [250, 250, 249], dim: [87, 83, 78], dust: [41, 37, 36] },
  wideMinPx: 760,
} as const;
