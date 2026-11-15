import { describe, expect, it } from 'vitest';
import { ASCII_FS, QUAD_VS, SCENE_FS, SCENE_VS } from '../shaders';
import { ASCII } from '../constants';

describe('shaders', () => {
  it('take their thresholds from the shared constants', () => {
    expect(ASCII_FS).toContain(`dEdge>${ASCII.edge.depth.toFixed(2)}`);
    expect(ASCII_FS).toContain(`nEdge>${ASCII.edge.normal.toFixed(2)}`);
    expect(ASCII_FS).toContain(`pow(clamp(L.r,0.0,1.0),${ASCII.lum.gamma.toFixed(2)})`);
    expect(ASCII_FS).toContain(`smoothstep(${ASCII.sweep.halfWidth.toFixed(2)},0.0`);
  });

  it('use the prototype lighting', () => {
    expect(SCENE_FS).toContain('40.0');
    expect(SCENE_FS).toContain('vec3(3.00,4.00,5.00)');
  });

  it('declare the attributes the renderer binds', () => {
    expect(SCENE_VS).toContain('attribute vec3 aPos');
    expect(SCENE_VS).toContain('attribute vec3 aNrm');
    expect(QUAD_VS).toContain('attribute vec2 aPos');
  });

  it('contain no integer literals where GLSL ES 1.0 needs floats', () => {
    // A bare "0.09" style constant must always keep its decimal point.
    for (const n of [ASCII.edge.depth, ASCII.edge.normal, ASCII.lum.gamma]) expect(n.toFixed(2)).toMatch(/\./);
  });
});
