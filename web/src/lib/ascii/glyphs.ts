import { ASCII } from './constants';

const RAMP_N = Array.from(ASCII.ramp).length;
const EDGES = Array.from(ASCII.edges) as Array<'|' | '/' | '-' | '\\'>;

/**
 * Index into the ramp for a cell's light and coverage. Mirrors ASCII_FS:
 * covered cells always get at least the first visible glyph.
 */
export function rampIndex(lum: number, coverage: number): number {
  const v = Math.min(Math.max(lum * coverage, 0), 0.999);
  const idx = Math.floor(v * RAMP_N);
  return coverage > 0.5 ? Math.max(idx, 1) : idx;
}

/** Outline glyph for an edge whose screen-space gradient points at `angleRad`. Mirrors ASCII_FS. */
export function edgeGlyph(angleRad: number): '|' | '/' | '-' | '\\' {
  const a = (((angleRad + Math.PI) % Math.PI) + Math.PI) % Math.PI;
  const P = Math.PI;
  if (a < P / 8 || a >= (7 * P) / 8) return EDGES[0];
  if (a < (3 * P) / 8) return EDGES[3];
  if (a < (5 * P) / 8) return EDGES[2];
  return EDGES[1];
}
