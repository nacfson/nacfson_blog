'use client';

import React, { useState } from 'react';
import { BenchmarkChartData, BenchmarkSeriesItem } from '../../lib/types';

// Monochrome stone ramp used when a series item has no explicit color.
// Kept dark enough for the white in-bar value label to stay legible.
const SERIES_FALLBACK_COLORS = ['#0c0a09', '#44403c', '#57534e', '#78716c'];

interface BenchmarkChartProps {
  data: BenchmarkChartData;
}

export const BenchmarkChart: React.FC<BenchmarkChartProps> = ({ data }) => {
  const [activeTabIdx, setActiveTabIdx] = useState(0);

  const hasTabs = data.tabs && data.tabs.length > 0;
  const currentTab = hasTabs ? data.tabs![activeTabIdx] : null;
  const series: BenchmarkSeriesItem[] = (currentTab?.series || data.series) || [];
  const unit = (currentTab?.unit || data.unit) || '';

  const maxValue = Math.max(...series.map((s) => s.value), 1);

  return (
    <div className="my-8 rounded-md border border-stone-200 bg-white p-6">
      {/* Title & Subtitle */}
      <div className="mb-4">
        <h3 className="text-base font-semibold text-stone-950">{data.title}</h3>
        {data.subtitle && (
          <p className="mt-1 text-xs text-stone-500">{data.subtitle}</p>
        )}
      </div>

      {/* Tabs if present */}
      {hasTabs && data.tabs!.length > 1 && (
        <div className="mb-6 flex flex-wrap gap-1.5 border-b border-stone-200 pb-3">
          {data.tabs!.map((tab, idx) => {
            const isActive = idx === activeTabIdx;
            return (
              <button
                key={tab.id || idx}
                type="button"
                onClick={() => setActiveTabIdx(idx)}
                className={`px-3 py-1 text-xs font-medium rounded-md border transition-all ${
                  isActive
                    ? 'bg-stone-950 text-stone-50 font-semibold border-stone-950'
                    : 'border-transparent text-stone-600 hover:text-stone-950 hover:bg-stone-100'
                }`}
              >
                {tab.label}
              </button>
            );
          })}
        </div>
      )}

      {/* SVG Bar Chart */}
      <div className="space-y-4 pt-2">
        {series.map((item, idx) => {
          const percentage = Math.min(100, Math.max(4, (item.value / maxValue) * 100));
          const color = item.color || SERIES_FALLBACK_COLORS[idx % SERIES_FALLBACK_COLORS.length];

          return (
            <div key={`${item.name}-${idx}`} className="group">
              <div className="flex justify-between items-baseline text-xs mb-1.5">
                <span className="font-medium text-stone-700">{item.name}</span>
                <span className="font-mono font-semibold text-stone-950">
                  {item.value} {unit}
                </span>
              </div>
              <div className="h-6 w-full overflow-hidden rounded bg-stone-100 p-0.5">
                <div
                  className="h-full rounded-sm transition-all duration-500 ease-out flex items-center justify-end pr-2"
                  style={{
                    width: `${percentage}%`,
                    backgroundColor: color,
                  }}
                >
                  {percentage > 25 && (
                    <span className="text-[10px] font-medium text-white select-none">
                      {item.value} {unit}
                    </span>
                  )}
                </div>
              </div>
            </div>
          );
        })}
      </div>
    </div>
  );
};
