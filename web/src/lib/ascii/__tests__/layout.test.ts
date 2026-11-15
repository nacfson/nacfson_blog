import { describe, expect, it } from 'vitest';
import { fitKnot, gridSize } from '../layout';
import { ASCII } from '../constants';

function visibleHalf(cssW: number, cssH: number) {
  const { distance, fovDeg } = ASCII.camera;
  const halfH = Math.tan((fovDeg * Math.PI) / 360) * distance;
  return { halfW: halfH * (cssW / cssH), halfH };
}

describe('fitKnot', () => {
  it('switches to the wide layout at 760px', () => {
    expect(fitKnot(759, 600).wide).toBe(false);
    expect(fitKnot(760, 600).wide).toBe(true);
  });

  it.each([
    [1440, 600],
    [390, 600],
    [760, 420],
  ])('keeps the knot inside the frame at %ix%i', (w, h) => {
    const f = fitKnot(w, h);
    const { halfW, halfH } = visibleHalf(w, h);
    const r = ASCII.knot.radius * f.scale;
    expect(Math.abs(f.x) + r).toBeLessThanOrEqual(halfW);
    expect(Math.abs(f.y) + r).toBeLessThanOrEqual(halfH);
  });

  it('places the knot on the right on wide screens and centred on narrow ones', () => {
    expect(fitKnot(1440, 600).x).toBeGreaterThan(0);
    expect(fitKnot(390, 600).x).toBe(0);
    expect(fitKnot(390, 600).y).toBeLessThan(0);
  });
});

describe('gridSize', () => {
  it('covers the canvas with whole cells', () => {
    expect(gridSize(100, 30, 8, 14)).toEqual({ cols: 13, rows: 3 });
  });

  it('never returns an empty grid', () => {
    expect(gridSize(0, 0, 8, 14)).toEqual({ cols: 1, rows: 1 });
  });
});
