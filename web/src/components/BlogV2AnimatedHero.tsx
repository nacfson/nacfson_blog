'use client';

import React, { useState, useEffect, useRef, useCallback } from 'react';
import { ScrollHeroData, ScrollHeroControl } from '../lib/types';

interface BlogV2AnimatedHeroProps {
  data: ScrollHeroData;
}

export const BlogV2AnimatedHero: React.FC<BlogV2AnimatedHeroProps> = ({ data }) => {
  const containerRef = useRef<HTMLDivElement>(null);
  const [isIntersecting, setIsIntersecting] = useState(true);
  const [scrollY, setScrollY] = useState(0);
  const [showControls, setShowControls] = useState(false);

  // Initialize control values from data.controls
  const initialControlValues = React.useMemo(() => {
    const vals: Record<string, number> = {};
    if (data.controls) {
      data.controls.forEach((ctrl) => {
        vals[ctrl.id] = ctrl.default;
      });
    }
    return vals;
  }, [data.controls]);

  const [controlValues, setControlValues] = useState<Record<string, number>>(initialControlValues);

  // IntersectionObserver to pause parallax updates when offscreen
  useEffect(() => {
    if (typeof window === 'undefined' || !containerRef.current) return;

    if (!('IntersectionObserver' in window)) {
      setIsIntersecting(true);
      return;
    }

    const observer = new IntersectionObserver(
      ([entry]) => {
        setIsIntersecting(entry.isIntersecting);
      },
      { threshold: 0 }
    );

    observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, []);

  // Scroll listener with requestAnimationFrame
  useEffect(() => {
    if (typeof window === 'undefined' || !isIntersecting) return;

    let rafId: number | null = null;

    const handleScroll = () => {
      if (rafId) return;
      rafId = requestAnimationFrame(() => {
        if (containerRef.current) {
          const rect = containerRef.current.getBoundingClientRect();
          // Distance scrolled past container top
          const offset = Math.max(0, -rect.top);
          setScrollY(offset);
        }
        rafId = null;
      });
    };

    window.addEventListener('scroll', handleScroll, { passive: true });
    handleScroll();

    return () => {
      window.removeEventListener('scroll', handleScroll);
      if (rafId) cancelAnimationFrame(rafId);
    };
  }, [isIntersecting]);

  const handleControlChange = (id: string, val: number) => {
    setControlValues((prev) => ({ ...prev, [id]: val }));
  };

  const handleReset = () => {
    setControlValues(initialControlValues);
  };

  const parallaxMultiplier = (controlValues['parallaxDepth'] ?? 65) / 50;

  return (
    <div
      ref={containerRef}
      role="region"
      aria-label={data.ariaLabel || 'Article hero'}
      className="relative w-full overflow-hidden rounded-md border border-stone-200 bg-stone-100 transition-all mb-8"
      style={{
        aspectRatio: data.aspectRatio ? data.aspectRatio.replace('/', ' / ') : '16 / 9',
        maxHeight: data.heightVh ? `${data.heightVh}vh` : '560px',
      }}
    >
      {/* Base Background Image */}
      {data.baseImage && (
        <img
          src={data.baseImage}
          alt="Hero background"
          className="absolute inset-0 h-full w-full object-cover object-center select-none"
        />
      )}

      {/* Parallax Layers */}
      {data.layers &&
        data.layers.map((layer, index) => {
          const depth = layer.depth ?? (layer.parallaxSpeed ? layer.parallaxSpeed / 2 : 0.3 * (index + 1));
          const translateY = scrollY * depth * parallaxMultiplier;
          const opacity = layer.fadeOnScroll
            ? Math.max(0, 1 - scrollY / 300)
            : 1;

          return (
            <img
              key={`${layer.src}-${index}`}
              src={layer.src}
              alt={layer.alt || `Hero layer ${index + 1}`}
              className="absolute inset-0 h-full w-full object-cover object-center pointer-events-none select-none transition-transform duration-75"
              style={{
                transform: `translate3d(0, ${translateY}px, 0)`,
                opacity,
              }}
            />
          );
        })}

      {/* Scroll Hint */}
      {data.scrollHint && (
        <div className="absolute bottom-4 left-1/2 -translate-x-1/2 flex items-center gap-1.5 px-3 py-1 rounded-full bg-white/80 backdrop-blur-sm border border-stone-200 text-xs font-medium text-stone-700 pointer-events-none">
          <span>{data.scrollHint}</span>
          <svg className="w-3.5 h-3.5 animate-bounce" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M19 14l-7 7m0 0l-7-7m7 7V3" />
          </svg>
        </div>
      )}

      {/* Interactive Controls Toggle & Panel */}
      <div className="absolute top-3 right-3 z-20">
        <button
          type="button"
          onClick={() => setShowControls((prev) => !prev)}
          className="px-2.5 py-1 text-xs font-medium text-stone-800 bg-white/90 hover:bg-white backdrop-blur-md border border-stone-300 rounded-md transition-colors flex items-center gap-1.5"
          aria-expanded={showControls}
        >
          <svg className="w-3.5 h-3.5 text-stone-950" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 6V4m0 2a2 2 0 100 4m0-4a2 2 0 110 4m-6 8a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4m6 6v10m6-2a2 2 0 100-4m0 4a2 2 0 110-4m0 4v2m0-6V4" />
          </svg>
          Interactive Layers
        </button>

        {showControls && (
          <div className="mt-2 w-64 p-3 bg-white/95 backdrop-blur-md rounded-md border border-stone-300 shadow-sm text-xs space-y-3">
            <div className="flex items-center justify-between border-b border-stone-200 pb-2">
              <span className="font-semibold text-stone-900">Layer Controls</span>
              <button
                type="button"
                onClick={handleReset}
                className="text-stone-950 font-medium underline-offset-4 hover:underline text-[11px]"
              >
                Reset Layers
              </button>
            </div>

            {data.controls && data.controls.length > 0 ? (
              data.controls.map((ctrl) => (
                <div key={ctrl.id} className="space-y-1">
                  <div className="flex justify-between text-stone-600 font-medium">
                    <label htmlFor={`ctrl-${ctrl.id}`}>{ctrl.label}</label>
                    <span className="font-mono text-stone-500">
                      {controlValues[ctrl.id] ?? ctrl.default}
                    </span>
                  </div>
                  <input
                    id={`ctrl-${ctrl.id}`}
                    type="range"
                    min={ctrl.min}
                    max={ctrl.max}
                    step={ctrl.step}
                    value={controlValues[ctrl.id] ?? ctrl.default}
                    onChange={(e) => handleControlChange(ctrl.id, parseFloat(e.target.value))}
                    className="w-full accent-stone-950 h-1.5 bg-stone-200 rounded cursor-pointer"
                  />
                </div>
              ))
            ) : (
              <div className="text-stone-500 italic">No custom sliders configured.</div>
            )}
          </div>
        )}
      </div>
    </div>
  );
};
