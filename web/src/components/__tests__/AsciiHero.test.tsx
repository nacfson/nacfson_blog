import React from 'react';
import { act, render, screen } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';

const fake = { resize: vi.fn(), frame: vi.fn(), dispose: vi.fn() };
const create = vi.fn();
vi.mock('../../lib/ascii/renderer', () => ({
  createAsciiRenderer: (...args: unknown[]) => create(...args),
}));

import AsciiHero from '../AsciiHero';

let idleQueue: Array<() => void> = [];
let rafQueue: Array<(t: number) => void> = [];
let clock = 0;
const cancelRaf = vi.fn();

function flushIdle() {
  const q = idleQueue;
  idleQueue = [];
  q.forEach((cb) => cb());
}

function tick(n: number) {
  for (let i = 0; i < n; i++) {
    const q = rafQueue;
    rafQueue = [];
    clock += 16;
    q.forEach((cb) => cb(clock));
  }
}

function reducedMotion(matches: boolean) {
  vi.stubGlobal('matchMedia', (query: string) => ({ matches: matches && query.includes('reduce'), media: query }));
}

beforeEach(() => {
  idleQueue = [];
  rafQueue = [];
  clock = performance.now();
  create.mockReset();
  fake.resize.mockReset();
  fake.frame.mockReset();
  fake.dispose.mockReset();
  cancelRaf.mockReset();
  vi.stubGlobal('requestIdleCallback', (cb: () => void) => {
    idleQueue.push(cb);
    return idleQueue.length;
  });
  vi.stubGlobal('cancelIdleCallback', () => {
    idleQueue = [];
  });
  vi.stubGlobal('requestAnimationFrame', (cb: (t: number) => void) => {
    rafQueue.push(cb);
    return rafQueue.length;
  });
  vi.stubGlobal('cancelAnimationFrame', cancelRaf);
  vi.stubGlobal(
    'IntersectionObserver',
    class {
      constructor(private cb: (e: Array<{ isIntersecting: boolean }>) => void) {}
      observe() {
        this.cb([{ isIntersecting: true }]);
      }
      disconnect() {}
    },
  );
  reducedMotion(false);
});

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

describe('AsciiHero', () => {
  it('shows the headline and a static knot, with no canvas, when WebGL is unavailable', async () => {
    const actual = await vi.importActual<typeof import('../../lib/ascii/renderer')>('../../lib/ascii/renderer');
    create.mockImplementation(actual.createAsciiRenderer);
    const errors = vi.spyOn(console, 'error');

    const { container } = render(<AsciiHero />);
    act(() => flushIdle());
    tick(2);

    expect(screen.getByRole('heading', { level: 1, name: 'Make Better World' })).toBeInTheDocument();
    expect(screen.getByText(/good engineering can make the world a little better/i)).toBeInTheDocument();
    expect(screen.getByRole('link', { name: 'Browse articles ↓' })).toHaveAttribute('href', '#articles');
    expect(container.querySelector('canvas')).toBeNull();
    // The live stage stays hidden and a still frame of the knot takes its place.
    expect(container.querySelector('section > div[hidden]')).not.toBeNull();
    expect(container.querySelector('section pre')?.textContent).toContain('###');
    expect(errors).not.toHaveBeenCalled();
  });

  it('mounts after first paint, draws frames and cleans up on unmount', () => {
    create.mockReturnValue(fake);
    const { container, unmount } = render(<AsciiHero />);

    expect(create).not.toHaveBeenCalled();
    flushIdle();
    expect(create).toHaveBeenCalledTimes(1);
    expect(container.querySelector('canvas')).toHaveAttribute('aria-hidden', 'true');
    expect(fake.resize).toHaveBeenCalled();

    tick(3);
    expect(fake.frame).toHaveBeenCalledTimes(3);

    unmount();
    expect(fake.dispose).toHaveBeenCalledTimes(1);
    expect(cancelRaf).toHaveBeenCalled();
  });

  it('draws a single still frame under reduced motion', () => {
    reducedMotion(true);
    create.mockReturnValue(fake);
    render(<AsciiHero />);
    flushIdle();
    tick(3);

    expect(fake.frame).toHaveBeenCalledTimes(1);
    const state = fake.frame.mock.calls[0][0];
    expect(state.intro).toBe(1);
    expect(state.sweep).toBe(-1);
  });

  it('does nothing if unmounted before the idle callback runs', () => {
    create.mockReturnValue(fake);
    const { unmount } = render(<AsciiHero />);
    const pending = idleQueue;
    unmount();
    pending.forEach((cb) => cb());

    expect(create).not.toHaveBeenCalled();
  });

  it('falls back to the plain headline when the renderer reports a failure later', () => {
    let onFail: (() => void) | undefined;
    create.mockImplementation((_canvas: HTMLCanvasElement, cb: () => void) => {
      onFail = cb;
      return fake;
    });
    const { container } = render(<AsciiHero />);
    flushIdle();
    expect(container.querySelector('canvas')).not.toBeNull();

    onFail?.();
    expect(container.querySelector('canvas')).toBeNull();
    expect(fake.dispose).toHaveBeenCalledTimes(1);
  });
});
