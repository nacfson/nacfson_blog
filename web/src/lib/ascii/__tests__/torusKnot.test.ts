import { describe, expect, it } from 'vitest';
import { torusKnot } from '../torusKnot';
import { ASCII } from '../constants';

describe('torusKnot', () => {
  const { R, tube, tubular, radial, p, q } = ASCII.knot;
  const g = torusKnot(R, tube, tubular, radial, p, q);
  const vertexCount = g.positions.length / 3;

  it('has the expected vertex and index counts', () => {
    expect(vertexCount).toBe(12341);
    expect(g.indices.length).toBe(72000);
    expect(g.normals.length).toBe(g.positions.length);
  });

  it('only references existing vertices', () => {
    let max = 0;
    for (const i of g.indices) max = Math.max(max, i);
    expect(max).toBeLessThan(vertexCount);
  });

  it('has unit-length normals', () => {
    for (let i = 0; i < vertexCount; i++) {
      const len = Math.hypot(g.normals[i * 3], g.normals[i * 3 + 1], g.normals[i * 3 + 2]);
      expect(Math.abs(len - 1)).toBeLessThan(1e-6);
    }
  });

  it('fits inside the radius used for layout', () => {
    let max = 0;
    for (let i = 0; i < vertexCount; i++) {
      max = Math.max(max, Math.hypot(g.positions[i * 3], g.positions[i * 3 + 1], g.positions[i * 3 + 2]));
    }
    expect(max).toBeLessThanOrEqual(2.05);
    expect(ASCII.knot.radius).toBeGreaterThanOrEqual(max - 0.05);
  });
});
