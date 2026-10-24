'use client';

import React, { useState } from 'react';
import { BenchmarkChartData, BenchmarkSeriesItem } from '../../lib/types';

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
    <div className="my-8 rounded-2xl border border-slate-200/90 bg-white p-6 shadow-sm">
      {/* Title & Subtitle */}
      <div className="mb-4">
        <h3 className="text-base font-semibold text-slate-900">{data.title}</h3>
        {data.subtitle && (
          <p className="mt-1 text-xs text-slate-500">{data.subtitle}</p>
        )}
      </div>

      {/* Tabs if present */}
      {hasTabs && data.tabs!.length > 1 && (
        <div className="mb-6 flex flex-wrap gap-1.5 border-b border-slate-100 pb-3">
          {data.tabs!.map((tab, idx) => {
            const isActive = idx === activeTabIdx;
            return (
              <button
                key={tab.id || idx}
                type="button"
                onClick={() => setActiveTabIdx(idx)}
                className={`px-3 py-1 text-xs font-medium rounded-lg transition-all ${
                  isActive
                    ? 'bg-blue-50 text-blue-700 font-semibold border border-blue-200/80 shadow-xs'
                    : 'text-slate-600 hover:text-slate-900 hover:bg-slate-100'
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
          const color = item.color || '#2563eb';

          return (
            <div key={`${item.name}-${idx}`} className="group">
              <div className="flex justify-between items-baseline text-xs mb-1.5">
                <span className="font-medium text-slate-700">{item.name}</span>
                <span className="font-mono font-semibold text-slate-900">
                  {item.value} {unit}
                </span>
              </div>
              <div className="h-6 w-full overflow-hidden rounded-lg bg-slate-100 p-0.5">
                <div
                  className="h-full rounded-md transition-all duration-500 ease-out flex items-center justify-end pr-2"
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
