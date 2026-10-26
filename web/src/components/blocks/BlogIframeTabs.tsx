'use client';

import React, { useState, useEffect, useRef } from 'react';
import { BlogIframeTabsData, IframeTab } from '../../lib/types';

interface BlogIframeTabsProps {
  data: BlogIframeTabsData;
}

export const BlogIframeTabs: React.FC<BlogIframeTabsProps> = ({ data }) => {
  const [activeTabIdx, setActiveTabIdx] = useState(0);
  const [mountedTabs, setMountedTabs] = useState<Record<number, boolean>>({});
  const [isFullscreen, setIsFullscreen] = useState(false);
  const containerRef = useRef<HTMLDivElement>(null);

  const tabs = data.tabs || [];
  const activeTab: IframeTab | undefined = tabs[activeTabIdx];

  const activeSrc = activeTab?.src || activeTab?.panels?.[0]?.src || '';
  const activePoster = activeTab?.poster || activeTab?.panels?.[0]?.poster || '';
  const interaction =
    activeTab?.panels?.[0]?.interaction || data.mountStrategy || 'click-or-center';

  const isEager = interaction === 'eager';
  const isMounted = isEager || !!mountedTabs[activeTabIdx];

  // Optional center observer for click-or-center
  useEffect(() => {
    if (isMounted || !containerRef.current || typeof window === 'undefined') return;

    if (!('IntersectionObserver' in window)) return;

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          // If centered in viewport
          if (entry.intersectionRatio >= 0.75) {
            setMountedTabs((prev) => ({ ...prev, [activeTabIdx]: true }));
          }
        });
      },
      {
        threshold: [0.75],
      }
    );

    observer.observe(containerRef.current);
    return () => observer.disconnect();
  }, [activeTabIdx, isMounted]);

  const handleLaunch = () => {
    setMountedTabs((prev) => ({ ...prev, [activeTabIdx]: true }));
  };

  const containerHeight = data.height || '500px';

  return (
    <div
      ref={containerRef}
      className={`my-8 overflow-hidden rounded-md border border-stone-200 bg-white transition-all ${
        isFullscreen ? 'fixed inset-0 z-50 m-0 rounded-none border-none p-4 bg-stone-950/90 backdrop-blur-md flex flex-col' : ''
      }`}
    >
      {/* Top Header: Tabs & Actions */}
      <div className="flex flex-wrap items-center justify-between border-b border-stone-200 bg-stone-50 px-4 py-2.5 gap-2">
        {/* Tabs */}
        <div className="flex items-center space-x-1 overflow-x-auto">
          {tabs.map((tab, idx) => {
            const isActive = idx === activeTabIdx;
            return (
              <button
                key={tab.id || idx}
                type="button"
                onClick={() => setActiveTabIdx(idx)}
                className={`px-3 py-1.5 text-xs font-medium rounded-md transition-all ${
                  isActive
                    ? 'bg-white text-stone-950 border border-stone-300 font-semibold'
                    : 'text-stone-600 hover:text-stone-950 hover:bg-stone-200/60'
                }`}
              >
                {tab.label}
              </button>
            );
          })}
        </div>

        {/* Action Controls: Fullscreen toggle & Standalone link */}
        <div className="flex items-center space-x-2">
          {activeTab?.cta && (
            <a
              href={activeTab.cta.url}
              target="_blank"
              rel="noopener noreferrer"
              className="text-xs text-stone-700 hover:text-stone-950 hover:underline underline-offset-4 font-medium flex items-center gap-1 px-2 py-1 hover:bg-stone-100 rounded"
            >
              <span>{activeTab.cta.label}</span>
              <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M10 6H6a2 2 0 00-2 2v10a2 2 0 002 2h10a2 2 0 002-2v-4M14 4h6m0 0v6m0-6L10 14" />
              </svg>
            </a>
          )}

          <button
            type="button"
            onClick={() => setIsFullscreen((prev) => !prev)}
            className="p-1.5 text-stone-500 hover:text-stone-950 hover:bg-stone-200/60 rounded-md transition-colors"
            title={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
            aria-label={isFullscreen ? 'Exit Fullscreen' : 'Fullscreen'}
          >
            {isFullscreen ? (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M6 18L18 6M6 6l12 12" />
              </svg>
            ) : (
              <svg className="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M4 8V4m0 0h4M4 4l5 5m11-5h-4m4 0v4m0-4l-5 5M4 16v4m0 0h4m-4 0l5-5m11 5l-5-5m5 5v-4m0 4h-4" />
              </svg>
            )}
          </button>
        </div>
      </div>

      {/* Main Body */}
      <div
        className={`relative w-full bg-stone-900 ${
          isFullscreen ? 'flex-1' : ''
        }`}
        style={{ height: isFullscreen ? '100%' : containerHeight }}
      >
        {isMounted ? (
          <iframe
            key={activeSrc}
            src={activeSrc}
            sandbox="allow-scripts"
            className="h-full w-full border-0 bg-white"
            title={activeTab?.label || 'Interactive Demo'}
          />
        ) : (
          <div className="relative h-full w-full overflow-hidden bg-stone-950 flex items-center justify-center">
            {activePoster ? (
              <img
                src={activePoster}
                alt={activeTab?.label || 'Demo poster'}
                className="absolute inset-0 h-full w-full object-contain opacity-80"
              />
            ) : (
              <div className="absolute inset-0 bg-stone-900" />
            )}

            {/* Launch Overlay Button */}
            <div className="relative z-10 flex flex-col items-center gap-3 p-4 text-center">
              <button
                type="button"
                onClick={handleLaunch}
                className="group flex items-center gap-2.5 rounded-md border border-white bg-white px-5 py-3 text-sm font-semibold text-stone-950 shadow-none transition-all hover:bg-stone-200 hover:scale-105 active:scale-95"
              >
                <svg className="w-5 h-5 fill-current transition-transform group-hover:scale-110" viewBox="0 0 24 24">
                  <path d="M8 5v14l11-7z" />
                </svg>
                Launch Interactive Demo
              </button>
              <span className="text-xs text-stone-300 backdrop-blur-sm px-2.5 py-1 rounded-md bg-black/40">
                Click to load live sandboxed HTML simulator
              </span>
            </div>
          </div>
        )}
      </div>

      {/* Caption description footer if present */}
      {(activeTab?.captionTitle || activeTab?.captionDescription || activeTab?.caption) && (
        <div className="border-t border-stone-200 bg-white px-4 py-3 text-xs text-stone-600">
          {activeTab.captionTitle && (
            <div className="font-semibold text-stone-900 mb-0.5">{activeTab.captionTitle}</div>
          )}
          <div>{activeTab.captionDescription || activeTab.caption}</div>
        </div>
      )}
    </div>
  );
};
