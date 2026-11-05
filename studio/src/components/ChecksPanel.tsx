import React from 'react';
import { Issue } from '../types';

interface ChecksPanelProps {
  issues: Issue[];
  onSelectIssue: (issue: Issue) => void;
}

export const ChecksPanel: React.FC<ChecksPanelProps> = ({ issues, onSelectIssue }) => {
  const errors = issues.filter((i) => i.severity === 'error');
  const warnings = issues.filter((i) => i.severity === 'warning');

  return (
    <div>
      <div className="pane-head">
        Checks{' '}
        <span className="count" data-testid="lint-count">
          {errors.length} error{errors.length === 1 ? '' : 's'} · {warnings.length} warning
          {warnings.length === 1 ? '' : 's'}
        </span>
      </div>
      <ul className="lint-list" aria-label="Checks list">
        {issues.length === 0 ? (
          <li>
            <span className="sev ok">✓</span>
            <span className="ok">Passes the production parser. Ready to publish.</span>
            <span />
          </li>
        ) : (
          issues.map((issue, idx) => {
            const clickable = Boolean(issue.line || issue.field);
            const loc = issue.field
              ? `frontmatter.${issue.field}`
              : issue.line
                ? `line ${issue.line}`
                : '';
            return (
              <li
                key={`${issue.severity}-${issue.field || ''}-${issue.line || 0}-${idx}`}
                data-clickable={clickable ? 'true' : undefined}
                data-line={issue.line}
                onClick={() => clickable && onSelectIssue(issue)}
              >
                <span className="sev">{issue.severity === 'error' ? '●' : '▲'}</span>
                <span>{issue.message}</span>
                <span className="loc">{loc}</span>
              </li>
            );
          })
        )}
      </ul>
    </div>
  );
};
