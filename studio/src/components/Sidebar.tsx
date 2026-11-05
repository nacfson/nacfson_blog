import React from 'react';
import { DraftView, PublishedPost } from '../types';
import { dotClassForState } from '../utils';

interface SidebarProps {
  publishBranch: string;
  remote: string;
  drafts: DraftView[];
  published: PublishedPost[];
  activeKey: string | null;
  activePublishedSlug: string | null;
  onNewDraft: () => void;
  onSelectDraft: (key: string) => void;
  onSelectPublished: (slug: string) => void;
}

export const Sidebar: React.FC<SidebarProps> = ({
  publishBranch,
  remote,
  drafts,
  published,
  activeKey,
  activePublishedSlug,
  onNewDraft,
  onSelectDraft,
  onSelectPublished,
}) => {
  const nonLiveDrafts = drafts.filter((d) => d.state !== 'live');

  return (
    <aside className="side">
      <button type="button" className="btn block" onClick={onNewDraft}>
        + New draft
      </button>

      <h3>Drafts</h3>
      <ul aria-label="Drafts">
        {nonLiveDrafts.length === 0 ? (
          <li className="empty">No drafts</li>
        ) : (
          nonLiveDrafts.map((d) => {
            const isSelected = activeKey === d.key;
            return (
              <li key={d.key}>
                <button
                  type="button"
                  className={isSelected ? 'active' : ''}
                  onClick={() => onSelectDraft(d.key)}
                >
                  <span className="t">{d.title || 'Untitled'}</span>
                  <span className="m">
                    <span className={dotClassForState(d.state)} />
                    {d.state} · {d.date || '—'}
                  </span>
                </button>
              </li>
            );
          })
        )}
      </ul>

      <h3>
        Published ·{' '}
        <span style={{ fontFamily: 'var(--mono)', textTransform: 'none', letterSpacing: 0 }}>
          {remote}/{publishBranch}
        </span>
      </h3>
      <ul aria-label="Published posts">
        {published.length === 0 ? (
          <li className="empty">Nothing published yet</li>
        ) : (
          published.map((p) => {
            const slug = p.summary.slug || p.path;
            const isSelected = activePublishedSlug === slug && !activeKey;
            return (
              <li key={p.path}>
                <button
                  type="button"
                  className={isSelected ? 'active' : ''}
                  onClick={() => onSelectPublished(p.summary.slug)}
                >
                  <span className="t">{p.summary.title || p.path}</span>
                  <span className="m">
                    <span className="dot pub" />
                    live · {p.summary.date || '—'}
                  </span>
                </button>
              </li>
            );
          })
        )}
      </ul>
    </aside>
  );
};
