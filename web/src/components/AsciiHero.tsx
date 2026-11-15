'use client';

import React, { useEffect, useRef } from 'react';
import { ASCII } from '../lib/ascii/constants';
import { createAsciiRenderer } from '../lib/ascii/renderer';

const LEDE =
  'I believe good engineering can make the world a little better every day. Here I share what I learn while building cloud-native platforms, zero-trust security, AI agents, and the open-source communities behind them.';

type IdleHandle = { cancel: () => void };

/** Runs `cb` once the browser is idle after first paint. */
function whenIdle(cb: () => void): IdleHandle {
  if (typeof window.requestIdleCallback === 'function') {
    const id = window.requestIdleCallback(cb, { timeout: 600 });
    return { cancel: () => window.cancelIdleCallback?.(id) };
  }
  const id = window.setTimeout(cb, 0);
  return { cancel: () => window.clearTimeout(id) };
}

/**
 * Starts the live knot inside `section`. Returns a teardown function.
 * If WebGL is unavailable the section keeps its static copy and nothing else happens.
 */
function start(section: HTMLElement): () => void {
  const canvas = document.createElement('canvas');
  canvas.setAttribute('aria-hidden', 'true');
  canvas.className = 'absolute inset-0 h-full w-full';

  let stopped = false;
  const renderer = createAsciiRenderer(canvas, () => stop());
  if (!renderer) return () => {};
  section.insertBefore(canvas, section.firstChild);

  const reduce = typeof window.matchMedia === 'function' && window.matchMedia('(prefers-reduced-motion: reduce)').matches;
  const m = ASCII.motion;
  let raf = 0;
  let last = performance.now();
  let T = 0;
  let visible = true;
  let dirty = true;
  let yaw = 0;
  let yawT = 0;
  let tiltX = 0;
  let tiltY = 0;
  let px = 0;
  let py = 0;
  let pointer: [number, number] | null = null;
  let dragging = false;
  let lastX = 0;

  const resize = () => {
    renderer.resize(section.clientWidth, section.clientHeight, window.devicePixelRatio || 1);
    dirty = true;
  };

  const locate = (e: PointerEvent) => {
    const r = canvas.getBoundingClientRect();
    const x = e.clientX - r.left;
    const y = e.clientY - r.top;
    px = r.width ? (x / r.width) * 2 - 1 : 0;
    py = r.height ? -((y / r.height) * 2 - 1) : 0;
    pointer = [x, y];
  };
  const onMove = (e: PointerEvent) => {
    if (e.pointerType === 'touch' && !dragging) return;
    locate(e);
    if (dragging) {
      yawT += ((e.clientX - lastX) / Math.max(section.clientWidth, 1)) * m.dragRadPerWidth;
      lastX = e.clientX;
    }
    dirty = true;
  };
  const onDown = (e: PointerEvent) => {
    if ((e.target as Element | null)?.closest?.('a')) return;
    dragging = true;
    lastX = e.clientX;
    locate(e);
    dirty = true;
  };
  const onUp = (e: PointerEvent) => {
    dragging = false;
    if (e.pointerType === 'touch') pointer = null;
    dirty = true;
  };
  const onLeave = () => {
    pointer = null;
    dirty = true;
  };
  section.addEventListener('pointermove', onMove);
  section.addEventListener('pointerdown', onDown);
  section.addEventListener('pointerleave', onLeave);
  window.addEventListener('pointerup', onUp);
  window.addEventListener('pointercancel', onUp);

  const io =
    typeof IntersectionObserver === 'function'
      ? new IntersectionObserver((entries) => {
          visible = entries[entries.length - 1]?.isIntersecting ?? true;
          dirty = true;
        })
      : null;
  io?.observe(section);
  const ro = typeof ResizeObserver === 'function' ? new ResizeObserver(resize) : null;
  ro?.observe(section);
  if (!ro) window.addEventListener('resize', resize);
  resize();

  const loop = (now: number) => {
    if (stopped) return;
    raf = window.requestAnimationFrame(loop);
    const dt = Math.min(0.05, Math.max(0, (now - last) / 1000));
    last = now;
    if (document.hidden || !visible) return;
    if (reduce && !dirty) return;
    if (!reduce) T += dt;
    const k = reduce ? 1 : 1 - Math.exp(-m.damping * dt);
    const hovering = pointer !== null;
    yaw += (yawT - yaw) * k;
    tiltX += ((hovering ? -py * m.tiltX : 0) - tiltX) * k;
    tiltY += ((hovering ? px * m.tiltY : 0) - tiltY) * k;
    const t = reduce ? ASCII.stillT : T;
    renderer.frame({
      t,
      intro: reduce ? 1 : Math.min(1, (T * 1000) / ASCII.intro.ms),
      sweep: reduce ? -1 : ((t / ASCII.sweep.periodS) % 1) * 1.6 - 0.3,
      yaw,
      tiltX,
      tiltY,
      pointer,
    });
    dirty = false;
  };
  // resize() above may already have failed and stopped the hero.
  if (!stopped) raf = window.requestAnimationFrame(loop);

  function stop() {
    if (stopped) return;
    stopped = true;
    window.cancelAnimationFrame(raf);
    io?.disconnect();
    ro?.disconnect();
    window.removeEventListener('resize', resize);
    section.removeEventListener('pointermove', onMove);
    section.removeEventListener('pointerdown', onDown);
    section.removeEventListener('pointerleave', onLeave);
    window.removeEventListener('pointerup', onUp);
    window.removeEventListener('pointercancel', onUp);
    renderer?.dispose();
    canvas.remove();
  }
  return stop;
}

/** Homepage hero: the headline on an ink band, with a live knot drawn in glyphs behind it. */
export default function AsciiHero() {
  const ref = useRef<HTMLElement>(null);

  useEffect(() => {
    const section = ref.current;
    if (!section) return;
    let cancelled = false;
    let stop: (() => void) | null = null;
    const idle = whenIdle(() => {
      if (!cancelled) stop = start(section);
    });
    return () => {
      cancelled = true;
      idle.cancel();
      stop?.();
    };
  }, []);

  return (
    <section
      ref={ref}
      aria-labelledby="hero-title"
      className="relative h-[600px] overflow-hidden border-b border-stone-200 bg-stone-950 min-[760px]:h-[clamp(420px,60vh,600px)]"
      style={{ touchAction: 'pan-y' }}
    >
      <div className="pointer-events-none relative z-10 mx-auto max-w-7xl px-4 pt-10 sm:px-6 min-[760px]:pt-16 lg:px-8">
        <div className="min-[760px]:max-w-[470px]">
          <p className="mb-3.5 font-mono text-[11px] uppercase tracking-[0.06em] text-stone-500">Rendered in text, live</p>
          <h1 id="hero-title" className="font-serif text-[40px] font-bold leading-none tracking-tight text-stone-50 sm:text-5xl lg:text-6xl">
            Make Better World
          </h1>
          <p className="mt-4 text-[15px] leading-relaxed text-stone-400">{LEDE}</p>
          <a
            href="#articles"
            className="pointer-events-auto mt-6 inline-block rounded-sm text-[13px] font-semibold text-stone-50 underline underline-offset-4 hover:no-underline focus-visible:outline focus-visible:outline-2 focus-visible:outline-offset-4 focus-visible:outline-stone-50"
          >
            Browse articles ↓
          </a>
        </div>
      </div>
    </section>
  );
}
