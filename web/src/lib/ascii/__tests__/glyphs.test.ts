import { describe, expect, it } from 'vitest';
import { edgeGlyph, rampIndex } from '../glyphs';
import { ASCII } from '../constants';

const N = Array.from(ASCII.ramp).length;

describe('rampIndex', () => {
  it('never leaves a covered cell empty', () => {
    expect(rampIndex(0, 1)).toBe(1);
  });

  it('leaves uncovered cells empty', () => {
    expect(rampIndex(0.5, 0)).toBe(0);
  });

  it('uses the densest glyph at full light', () => {
    expect(rampIndex(1, 1)).toBe(N - 1);
  });

  it('is monotonic in light', () => {
    let prev = -1;
    for (let i = 0; i <= 100; i++) {
      const idx = rampIndex(i / 100, 1);
      expect(idx).toBeGreaterThanOrEqual(prev);
      prev = idx;
    }
  });
});

describe('edgeGlyph', () => {
  it('picks the glyph that follows the edge direction', () => {
    expect(edgeGlyph(0)).toBe('|');
    expect(edgeGlyph(Math.PI / 4)).toBe('\\');
    expect(edgeGlyph(Math.PI / 2)).toBe('-');
    expect(edgeGlyph((3 * Math.PI) / 4)).toBe('/');
  });

  it('treats opposite gradients the same', () => {
    for (const a of [0, 0.3, 1.1, 2, 2.9]) expect(edgeGlyph(a + Math.PI)).toBe(edgeGlyph(a));
    expect(edgeGlyph(-Math.PI / 4)).toBe('/');
  });
});
