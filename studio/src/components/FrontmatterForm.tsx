import React from 'react';
import { Topic } from '@web/lib/types';
import { Frontmatter, Issue } from '../types';

interface FrontmatterFormProps {
  frontmatter: Frontmatter;
  slugTouched: boolean;
  issues: Issue[];
  disabled?: boolean;
  titleInputRef?: React.RefObject<HTMLInputElement>;
  onChange: (next: Frontmatter, slugManuallyEdited?: boolean) => void;
}

export const FrontmatterForm: React.FC<FrontmatterFormProps> = ({
  frontmatter,
  slugTouched,
  issues,
  disabled,
  titleInputRef,
  onChange,
}) => {
  const hasError = (field: string) =>
    issues.some((i) => i.severity === 'error' && i.field === field);

  const tagsStr = frontmatter.tags.join(', ');

  return (
    <section className="fm" aria-label="Frontmatter">
      <label className="wide2">
        Title
        <input
          ref={titleInputRef}
          id="f-title"
          aria-label="Title"
          className={`title-in${hasError('title') ? ' bad' : ''}`}
          autoComplete="off"
          disabled={disabled}
          value={frontmatter.title}
          onChange={(e) => onChange({ ...frontmatter, title: e.target.value }, false)}
        />
      </label>

      <label>
        <span>
          Slug <span className="hint">{slugTouched ? '' : '(auto)'}</span>
        </span>
        <input
          id="f-slug"
          aria-label="Slug"
          className={hasError('slug') ? 'bad' : ''}
          autoComplete="off"
          disabled={disabled}
          value={frontmatter.slug}
          onChange={(e) => onChange({ ...frontmatter, slug: e.target.value }, true)}
        />
      </label>

      <label>
        Date
        <input
          id="f-date"
          aria-label="Date"
          type="date"
          className={hasError('date') ? 'bad' : ''}
          disabled={disabled}
          value={frontmatter.date}
          onChange={(e) => onChange({ ...frontmatter, date: e.target.value }, false)}
        />
      </label>

      <label>
        Topic
        <select
          id="f-topic"
          aria-label="Topic"
          className={hasError('topic') ? 'bad' : ''}
          disabled={disabled}
          value={frontmatter.topic}
          onChange={(e) =>
            onChange({ ...frontmatter, topic: e.target.value as Topic | '' }, false)
          }
        >
          <option value="">Choose…</option>
          <option value="cloud-native">Cloud Native</option>
          <option value="zero-trust">Zero Trust</option>
          <option value="ai-agents">AI Agents</option>
          <option value="open-source">Open Source</option>
        </select>
      </label>

      <label className="wide">
        Summary
        <input
          id="f-summary"
          aria-label="Summary"
          className={hasError('summary') ? 'bad' : ''}
          autoComplete="off"
          disabled={disabled}
          placeholder="One or two sentences for the card and article deck"
          value={frontmatter.summary}
          onChange={(e) => onChange({ ...frontmatter, summary: e.target.value }, false)}
        />
      </label>

      <label>
        Project
        <input
          id="f-project"
          aria-label="Project"
          className={hasError('project') ? 'bad' : ''}
          autoComplete="off"
          disabled={disabled}
          placeholder="optional"
          value={frontmatter.project}
          onChange={(e) => onChange({ ...frontmatter, project: e.target.value }, false)}
        />
      </label>

      <label>
        Tags
        <input
          id="f-tags"
          aria-label="Tags"
          className={hasError('tags') ? 'bad' : ''}
          autoComplete="off"
          disabled={disabled}
          placeholder="comma, separated"
          value={tagsStr}
          onChange={(e) => {
            const raw = e.target.value;
            const tags = raw
              .split(',')
              .map((t) => t.trim())
              .filter(Boolean);
            onChange({ ...frontmatter, tags }, false);
          }}
        />
      </label>
    </section>
  );
};
