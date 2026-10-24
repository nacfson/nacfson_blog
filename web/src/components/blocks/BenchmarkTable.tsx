'use client';

import React from 'react';
import { BenchmarkTableData, BenchmarkTableRowObj } from '../../lib/types';

interface BenchmarkTableProps {
  data: BenchmarkTableData;
}

export const BenchmarkTable: React.FC<BenchmarkTableProps> = ({ data }) => {
  const tableTitle = data.caption || data.title;

  const normalizeRow = (row: string[] | BenchmarkTableRowObj): string[] => {
    if (Array.isArray(row)) return row;
    return [row.metric, ...(row.values || [])];
  };

  const renderCellContent = (rawVal: string, isHighlightedCol: boolean) => {
    if (typeof rawVal !== 'string') return rawVal;

    // Check for **bold** pattern
    const boldMatch = rawVal.match(/^\*\*(.*?)\*\*$/);
    if (boldMatch) {
      return (
        <span className="inline-flex items-center px-2 py-0.5 rounded-md text-xs font-bold bg-blue-100 text-blue-800 border border-blue-200">
          {boldMatch[1]}
        </span>
      );
    }

    return (
      <span className={isHighlightedCol ? 'font-semibold text-blue-950' : 'text-slate-700'}>
        {rawVal}
      </span>
    );
  };

  return (
    <div className="my-8 overflow-hidden rounded-2xl border border-slate-200/90 bg-white shadow-sm">
      {tableTitle && (
        <div className="border-b border-slate-200 bg-slate-50/75 px-5 py-3">
          <h4 className="text-xs font-semibold uppercase tracking-wider text-slate-700">
            {tableTitle}
          </h4>
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="bg-slate-50/50 text-slate-600 font-semibold border-b border-slate-100">
            <tr>
              {data.columns.map((col, idx) => {
                const isHighlight = data.highlightColumn === idx;
                return (
                  <th
                    key={col}
                    scope="col"
                    className={`px-5 py-3 ${
                      isHighlight ? 'bg-blue-50/60 text-blue-900 font-bold border-x border-blue-100' : ''
                    }`}
                  >
                    {col}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody className="divide-y divide-slate-100">
            {data.rows.map((rowItem, rowIdx) => {
              const cells = normalizeRow(rowItem);
              return (
                <tr
                  key={rowIdx}
                  className="hover:bg-slate-50/60 transition-colors"
                >
                  {cells.map((cell, cellIdx) => {
                    const isHighlight = data.highlightColumn === cellIdx;
                    return (
                      <td
                        key={cellIdx}
                        className={`px-5 py-3.5 ${
                          cellIdx === 0 ? 'font-medium text-slate-900' : ''
                        } ${isHighlight ? 'bg-blue-50/30 border-x border-blue-100/50' : ''}`}
                      >
                        {renderCellContent(cell, isHighlight)}
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
};
