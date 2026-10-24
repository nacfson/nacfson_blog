'use client';

import React from 'react';
import { BlogV2AnimatedHero } from './BlogV2AnimatedHero';
import { BlogPicture } from './blocks/BlogPicture';
import { BlogIframeTabs } from './blocks/BlogIframeTabs';
import { BenchmarkChart } from './blocks/BenchmarkChart';
import { BenchmarkTable } from './blocks/BenchmarkTable';
import {
  ScrollHeroData,
  BlogPictureData,
  BlogIframeTabsData,
  BenchmarkChartData,
  BenchmarkTableData,
} from '../lib/types';

interface MarkdownDispatcherProps {
  markdown: string;
}

// Helper to render inline formatting: code, bold, links
function renderInline(text: string): React.ReactNode[] {
  // Regex to match code `foo`, bold **bar**, link [text](url)
  const regex = /(`[^`]+`|\*\*[^*]+\*\*|\[[^\]]+\]\([^)]+\))/g;
  const parts = text.split(regex);

  return parts.map((part, i) => {
    if (part.startsWith('`') && part.endsWith('`')) {
      return (
        <code
          key={i}
          className="rounded bg-slate-100 px-1.5 py-0.5 font-mono text-xs text-slate-800 border border-slate-200/80"
        >
          {part.slice(1, -1)}
        </code>
      );
    }
    if (part.startsWith('**') && part.endsWith('**')) {
      return (
        <strong key={i} className="font-semibold text-slate-900">
          {part.slice(2, -2)}
        </strong>
      );
    }
    const linkMatch = part.match(/^\[(.*?)\]\((.*?)\)$/);
    if (linkMatch) {
      return (
        <a
          key={i}
          href={linkMatch[2]}
          className="font-medium text-blue-600 underline hover:text-blue-700 transition-colors"
          target={linkMatch[2].startsWith('http') ? '_blank' : undefined}
          rel={linkMatch[2].startsWith('http') ? 'noopener noreferrer' : undefined}
        >
          {linkMatch[1]}
        </a>
      );
    }
    return part;
  });
}

function renderProseBlock(block: string, key: number | string): React.ReactNode {
  const trimmed = block.trim();
  if (!trimmed) return null;

  // Heading 1
  if (trimmed.startsWith('# ')) {
    return (
      <h1 key={key} className="mt-10 mb-4 text-3xl font-extrabold tracking-tight text-slate-900">
        {renderInline(trimmed.replace(/^#\s+/, ''))}
      </h1>
    );
  }

  // Heading 2
  if (trimmed.startsWith('## ')) {
    const raw = trimmed.replace(/^##\s+/, '');
    const id = raw.toLowerCase().replace(/[^a-z0-9]+/g, '-').replace(/(^-|-$)/g, '');
    return (
      <h2
        key={key}
        id={id}
        className="mt-10 mb-4 text-xl font-bold tracking-tight text-slate-900 scroll-mt-24 border-b border-slate-100 pb-2"
      >
        {renderInline(raw)}
      </h2>
    );
  }

  // Heading 3
  if (trimmed.startsWith('### ')) {
    return (
      <h3 key={key} className="mt-8 mb-3 text-lg font-semibold text-slate-800">
        {renderInline(trimmed.replace(/^###\s+/, ''))}
      </h3>
    );
  }

  // Blockquote
  if (trimmed.startsWith('>')) {
    const quoteText = trimmed
      .split('\n')
      .map((l) => l.replace(/^>\s?/, ''))
      .join('\n');
    return (
      <blockquote
        key={key}
        className="my-6 border-l-4 border-blue-500 bg-slate-50/80 px-4 py-3 italic text-slate-700 rounded-r-xl"
      >
        {renderInline(quoteText)}
      </blockquote>
    );
  }

  // Unordered list
  if (trimmed.startsWith('- ') || trimmed.startsWith('* ')) {
    const items = trimmed.split('\n').filter((l) => l.startsWith('- ') || l.startsWith('* '));
    return (
      <ul key={key} className="my-4 list-disc pl-6 space-y-1.5 text-sm text-slate-700">
        {items.map((item, itemIdx) => (
          <li key={itemIdx}>{renderInline(item.replace(/^[-*]\s+/, ''))}</li>
        ))}
      </ul>
    );
  }

  // Standard paragraph
  return (
    <p key={key} className="my-4 text-sm leading-relaxed text-slate-700 font-normal">
      {renderInline(trimmed)}
    </p>
  );
}

export const MarkdownDispatcher: React.FC<MarkdownDispatcherProps> = ({ markdown }) => {
  // Match code fences ```lang\ncode\n```
  // We use regex with capturing groups to tokenize into code blocks and prose chunks
  const fenceRegex = /```([a-zA-Z0-9_-]*)\n([\s\S]*?)```/g;

  const elements: React.ReactNode[] = [];
  let lastIndex = 0;
  let match: RegExpExecArray | null;
  let blockKey = 0;

  while ((match = fenceRegex.exec(markdown)) !== null) {
    const matchStart = match.index;
    const matchEnd = fenceRegex.lastIndex;

    // Prose before this code block
    if (matchStart > lastIndex) {
      const prose = markdown.slice(lastIndex, matchStart);
      const blocks = prose.split(/\n\s*\n/);
      blocks.forEach((b) => {
        const node = renderProseBlock(b, `prose-${blockKey++}`);
        if (node) elements.push(node);
      });
    }

    const lang = match[1]?.trim() || '';
    const rawContent = match[2];

    if (lang === 'scrollHero') {
      try {
        const data = JSON.parse(rawContent) as ScrollHeroData;
        elements.push(<BlogV2AnimatedHero key={`hero-${blockKey++}`} data={data} />);
      } catch (err) {
        elements.push(
          <div
            key={`err-${blockKey++}`}
            role="alert"
            className="my-4 rounded-xl border border-red-200 bg-red-50 p-4 text-xs font-mono text-red-700"
          >
            Invalid scrollHero JSON block
          </div>
        );
      }
    } else if (lang === 'blogPicture') {
      try {
        const data = JSON.parse(rawContent) as BlogPictureData;
        elements.push(<BlogPicture key={`pic-${blockKey++}`} data={data} />);
      } catch (err) {
        elements.push(
          <div
            key={`err-${blockKey++}`}
            role="alert"
            className="my-4 rounded-xl border border-red-200 bg-red-50 p-4 text-xs font-mono text-red-700"
          >
            Invalid blogPicture JSON block
          </div>
        );
      }
    } else if (lang === 'blogIframeTabs') {
      try {
        const data = JSON.parse(rawContent) as BlogIframeTabsData;
        elements.push(<BlogIframeTabs key={`tabs-${blockKey++}`} data={data} />);
      } catch (err) {
        elements.push(
          <div
            key={`err-${blockKey++}`}
            role="alert"
            className="my-4 rounded-xl border border-red-200 bg-red-50 p-4 text-xs font-mono text-red-700"
          >
            Invalid blogIframeTabs JSON block
          </div>
        );
      }
    } else if (lang === 'benchmarkChart') {
      try {
        const data = JSON.parse(rawContent) as BenchmarkChartData;
        elements.push(<BenchmarkChart key={`chart-${blockKey++}`} data={data} />);
      } catch (err) {
        elements.push(
          <div
            key={`err-${blockKey++}`}
            role="alert"
            className="my-4 rounded-xl border border-red-200 bg-red-50 p-4 text-xs font-mono text-red-700"
          >
            Invalid benchmarkChart JSON block
          </div>
        );
      }
    } else if (lang === 'benchmarkTable') {
      try {
        const data = JSON.parse(rawContent) as BenchmarkTableData;
        elements.push(<BenchmarkTable key={`tbl-${blockKey++}`} data={data} />);
      } catch (err) {
        elements.push(
          <div
            key={`err-${blockKey++}`}
            role="alert"
            className="my-4 rounded-xl border border-red-200 bg-red-50 p-4 text-xs font-mono text-red-700"
          >
            Invalid benchmarkTable JSON block
          </div>
        );
      }
    } else {
      // Standard generic code block
      elements.push(
        <pre
          key={`code-${blockKey++}`}
          className="my-6 overflow-x-auto rounded-xl border border-slate-800 bg-slate-900 p-4 font-mono text-xs text-slate-100"
        >
          <code>{rawContent}</code>
        </pre>
      );
    }

    lastIndex = matchEnd;
  }

  // Trailing prose after the last code block
  if (lastIndex < markdown.length) {
    const trailing = markdown.slice(lastIndex);
    const blocks = trailing.split(/\n\s*\n/);
    blocks.forEach((b) => {
      const node = renderProseBlock(b, `prose-${blockKey++}`);
      if (node) elements.push(node);
    });
  }

  return <div className="markdown-body space-y-2">{elements}</div>;
};
