import { afterEach, describe, expect, it, vi } from 'vitest';
import { createAsciiRenderer } from '../renderer';

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('createAsciiRenderer', () => {
  it('returns null without touching the canvas when the browser has no WebGL', () => {
    const errors = vi.spyOn(console, 'error');
    const canvas = document.createElement('canvas');
    const getContext = vi.spyOn(canvas, 'getContext');
    vi.stubGlobal('WebGLRenderingContext', undefined);

    expect(createAsciiRenderer(canvas)).toBeNull();
    expect(getContext).not.toHaveBeenCalled();
    expect(errors).not.toHaveBeenCalled();
  });

  it('returns null when a WebGL context cannot be created', () => {
    const errors = vi.spyOn(console, 'error');
    const canvas = document.createElement('canvas');
    vi.spyOn(canvas, 'getContext').mockReturnValue(null);
    vi.stubGlobal('WebGLRenderingContext', function WebGLRenderingContext() {});

    expect(createAsciiRenderer(canvas)).toBeNull();
    expect(errors).not.toHaveBeenCalled();
  });

  it('returns null instead of throwing when setup fails', () => {
    const errors = vi.spyOn(console, 'error');
    const canvas = document.createElement('canvas');
    const broken = new Proxy(
      {},
      {
        get() {
          throw new Error('lost');
        },
      },
    );
    vi.spyOn(canvas, 'getContext').mockReturnValue(broken as unknown as RenderingContext);
    vi.stubGlobal('WebGLRenderingContext', function WebGLRenderingContext() {});

    expect(createAsciiRenderer(canvas)).toBeNull();
    expect(errors).not.toHaveBeenCalled();
  });
});
