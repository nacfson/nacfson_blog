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
        <span className="inline-flex items-center px-2 py-0.5 rounded text-xs font-bold bg-stone-950 text-stone-50 border border-stone-950">
          {boldMatch[1]}
        </span>
      );
    }

    return (
      <span className={isHighlightedCol ? 'font-semibold text-stone-950' : 'text-stone-700'}>
        {rawVal}
      </span>
    );
  };

  return (
    <div className="my-8 overflow-hidden rounded-md border border-stone-200 bg-white">
      {tableTitle && (
        <div className="border-b border-stone-200 bg-stone-50 px-5 py-3">
          <h4 className="text-xs font-semibold uppercase tracking-wider text-stone-700">
            {tableTitle}
          </h4>
        </div>
      )}

      <div className="overflow-x-auto">
        <table className="w-full text-left text-xs">
          <thead className="bg-stone-50/50 text-stone-600 font-semibold border-b border-stone-200">
            <tr>
              {data.columns.map((col, idx) => {
                const isHighlight = data.highlightColumn === idx;
                return (
                  <th
                    key={col}
                    scope="col"
                    className={`px-5 py-3 ${
                      isHighlight ? 'bg-stone-100 text-stone-950 font-bold border-x border-stone-200' : ''
                    }`}
                  >
                    {col}
                  </th>
                );
              })}
            </tr>
          </thead>
          <tbody className="divide-y divide-stone-100">
            {data.rows.map((rowItem, rowIdx) => {
              const cells = normalizeRow(rowItem);
              return (
                <tr
                  key={rowIdx}
                  className="hover:bg-stone-50 transition-colors"
                >
                  {cells.map((cell, cellIdx) => {
                    const isHighlight = data.highlightColumn === cellIdx;
                    return (
                      <td
                        key={cellIdx}
                        className={`px-5 py-3.5 ${
                          cellIdx === 0 ? 'font-medium text-stone-950' : ''
                        } ${isHighlight ? 'bg-stone-50 border-x border-stone-200/70' : ''}`}
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
