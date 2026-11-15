import { ASCII } from './constants';

export type KnotFit = { scale: number; x: number; y: number; wide: boolean };

/**
 * Scale and position (world units at z = 0) for the knot inside a stage of the given CSS size.
 * Wide: the stage is the whole band and the knot sits right of the copy.
 * Narrow: the stage is its own strip below the copy, so the knot is centred in it.
 */
export function fitKnot(cssW: number, cssH: number): KnotFit {
  const { distance, fovDeg } = ASCII.camera;
  const vh = 2 * Math.tan((fovDeg * Math.PI) / 360) * distance;
  const vw = vh * (cssW / Math.max(cssH, 1));
  const wide = cssW >= ASCII.wideMinPx;
  const size = wide ? Math.min(vh * 0.37, vw * 0.22) : Math.min(vh * 0.36, vw * 0.4);
  return {
    scale: size / ASCII.knot.radius,
    x: wide ? vw * 0.2 : 0,
    y: wide ? -vh * 0.02 : 0,
    wide,
  };
}

/** Number of glyph cells needed to cover a canvas of w × h device pixels. */
export function gridSize(w: number, h: number, cw: number, ch: number): { cols: number; rows: number } {
  return { cols: Math.max(1, Math.ceil(w / cw)), rows: Math.max(1, Math.ceil(h / ch)) };
}
