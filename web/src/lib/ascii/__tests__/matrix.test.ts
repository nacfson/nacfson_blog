import { describe, expect, it } from 'vitest';
import { identity, multiply, normalMatrix3, perspective, rotateX, rotateY, scale, type Mat4 } from '../matrix';

function apply(m: Mat4, v: [number, number, number, number]): number[] {
  const out = [0, 0, 0, 0];
  for (let r = 0; r < 4; r++) for (let c = 0; c < 4; c++) out[r] += m[c * 4 + r] * v[c];
  return out;
}

describe('matrix', () => {
  it('identity is neutral for multiply', () => {
    const m = rotateY(0.7);
    expect(Array.from(multiply(identity(), m))).toEqual(Array.from(m));
    expect(Array.from(multiply(m, identity()))).toEqual(Array.from(m));
  });

  it('rotateX(pi/2) maps +Y to +Z', () => {
    const v = apply(rotateX(Math.PI / 2), [0, 1, 0, 0]);
    expect(v[0]).toBeCloseTo(0, 6);
    expect(v[1]).toBeCloseTo(0, 6);
    expect(v[2]).toBeCloseTo(1, 6);
  });

  it('perspective maps the near plane to -1 and the far plane to +1', () => {
    const p = perspective(1, 1.5, 0.1, 50);
    const near = apply(p, [0, 0, -0.1, 1]);
    const far = apply(p, [0, 0, -50, 1]);
    expect(near[2] / near[3]).toBeCloseTo(-1, 5);
    expect(far[2] / far[3]).toBeCloseTo(1, 5);
  });

  it('normalMatrix3 keeps the rotation of a uniformly scaled matrix', () => {
    const n = normalMatrix3(multiply(scale(2), rotateY(0.5)));
    const r = rotateY(0.5);
    for (let c = 0; c < 3; c++) {
      const col = [n[c * 3], n[c * 3 + 1], n[c * 3 + 2]];
      const len = Math.hypot(col[0], col[1], col[2]);
      for (let k = 0; k < 3; k++) expect(col[k] / len).toBeCloseTo(r[c * 4 + k], 6);
    }
  });
});
