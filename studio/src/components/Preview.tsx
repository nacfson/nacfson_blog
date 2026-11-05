import React from 'react';
import { MarkdownDispatcher } from '@web/components/MarkdownDispatcher';
import { topicLabel } from '@web/lib/topics';
import { Topic } from '@web/lib/types';
import { Frontmatter } from '../types';
import { estimateReadMinutes } from '../utils';

interface PreviewProps {
  frontmatter: Frontmatter;
  markdown: string;
}

const VALID_TOPICS = new Set<string>(['cloud-native', 'zero-trust', 'ai-agents', 'open-source']);

export const Preview: React.FC<PreviewProps> = ({ frontmatter, markdown }) => {
  const readMins = frontmatter.readTimeMinutes || estimateReadMinutes(markdown);
  const hasValidTopic = VALID_TOPICS.has(frontmatter.topic);

  return (
    <div className="pane">
      <div className="pane-head">
        Preview <span className="muted">MarkdownDispatcher · same renderer as blog.nacfson.cloud</span>
      </div>
      <div className="preview" data-testid="studio-preview">
        <div className="mb-3 flex items-center justify-between">
          <div className="flex items-center gap-3 text-xs text-stone-500 font-mono">
            <time dateTime={frontmatter.date}>{frontmatter.date || 'YYYY-MM-DD'}</time>
            <span>&middot;</span>
            <span>{readMins} min read</span>
          </div>

          <div className="flex items-center gap-2">
            {hasValidTopic && (
              <span className="px-2.5 py-0.5 rounded text-[10px] font-semibold uppercase tracking-wider bg-white text-stone-700 border border-stone-300">
                {topicLabel(frontmatter.topic as Topic)}
              </span>
            )}
            {frontmatter.project && (
              <span className="px-2.5 py-0.5 rounded text-[10px] font-medium bg-stone-100 text-stone-600">
                {frontmatter.project}
              </span>
            )}
          </div>
        </div>

        <header className="mb-6 max-w-4xl space-y-2.5">
          <h1 className="font-serif text-2xl sm:text-3xl font-bold tracking-tight text-stone-950 leading-tight">
            {frontmatter.title || 'Untitled'}
          </h1>

          {frontmatter.summary && (
            <p className="text-sm text-stone-600 leading-relaxed pt-0.5">{frontmatter.summary}</p>
          )}

          {frontmatter.tags.length > 0 && (
            <div className="flex flex-wrap gap-1.5 pt-1">
              {frontmatter.tags.map((tag) => (
                <span
                  key={tag}
                  className="px-2 py-0.5 rounded text-[10px] font-medium bg-stone-100 text-stone-600 font-mono"
                >
                  #{tag}
                </span>
              ))}
            </div>
          )}
        </header>

        <article className="max-w-none">
          <MarkdownDispatcher markdown={markdown} />
        </article>
      </div>
    </div>
  );
};
