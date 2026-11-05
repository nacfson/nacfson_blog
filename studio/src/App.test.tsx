import React from 'react';
import { render, screen, fireEvent } from '@testing-library/react';
import { describe, it, expect, vi } from 'vitest';
import { FrontmatterForm } from './components/FrontmatterForm';
import { ChecksPanel } from './components/ChecksPanel';
import { PublishDialog } from './components/PublishDialog';
import { Preview } from './components/Preview';
import {
  formatHeaderStatus,
  serializeFrontmatterPreview,
  slugify,
} from './utils';
import { Frontmatter, Issue, PublishPlan } from './types';

describe('Blog Studio utilities & components', () => {
  it('derives slugs from titles and formats status labels per spec §3.4', () => {
    expect(slugify('Zero-Plaintext K3s: Cilium & OpenBao!')).toBe(
      'zero-plaintext-k3s-cilium-openbao',
    );
    expect(formatHeaderStatus('unsaved', 0, 'main', '14:22:09')).toBe(
      'Unsaved changes · autosaved to worktree 14:22:09',
    );
    expect(formatHeaderStatus('new', 0, 'main', null)).toBe(
      'New draft · nothing committed yet',
    );
    expect(formatHeaderStatus('saved', 2, 'main', null)).toBe(
      'Saved locally · 2 commits not on main',
    );
    expect(formatHeaderStatus('live', 0, 'main', null)).toBe(
      'Published · in sync with origin/main',
    );
  });

  it('serializes frontmatter in fixed order without escaping inner quotes', () => {
    const fm: Frontmatter = {
      title: 'A "Quoted" Title',
      slug: 'quoted-title',
      date: '2026-11-04',
      topic: 'zero-trust',
      project: 'nacfson_pipeline',
      summary: 'Short summary',
      tags: ['cilium', 'openbao'],
    };
    const out = serializeFrontmatterPreview(fm, '## Section\n\nHello.\n');
    expect(out).toBe(
      [
        '---',
        'title: "A "Quoted" Title"',
        'slug: "quoted-title"',
        'date: "2026-11-04"',
        'topic: "zero-trust"',
        'project: "nacfson_pipeline"',
        'summary: "Short summary"',
        'tags: ["cilium", "openbao"]',
        '---',
        '',
        '## Section',
        '',
        'Hello.',
        '',
      ].join('\n'),
    );
  });

  it('tracks auto vs manual slug edits and highlights frontmatter errors', () => {
    const fm: Frontmatter = {
      title: '',
      slug: 'untitled',
      date: '2026-11-04',
      topic: '',
      project: '',
      summary: '',
      tags: [],
    };
    const issues: Issue[] = [
      { severity: 'error', field: 'title', message: 'title is required' },
      { severity: 'error', field: 'topic', message: 'topic is required' },
    ];
    const onChange = vi.fn();

    const { rerender } = render(
      <FrontmatterForm
        frontmatter={fm}
        slugTouched={false}
        issues={issues}
        onChange={onChange}
      />,
    );

    expect(screen.getByText('(auto)')).toBeInTheDocument();
    const titleInput = screen.getByLabelText('Title');
    expect(titleInput).toHaveClass('bad');

    fireEvent.change(titleInput, { target: { value: 'My New Post' } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ title: 'My New Post' }),
      false,
    );

    const slugInput = screen.getByLabelText(/Slug/);
    fireEvent.change(slugInput, { target: { value: 'custom-slug' } });
    expect(onChange).toHaveBeenCalledWith(
      expect.objectContaining({ slug: 'custom-slug' }),
      true,
    );

    rerender(
      <FrontmatterForm
        frontmatter={{ ...fm, slug: 'custom-slug' }}
        slugTouched={true}
        issues={[]}
        onChange={onChange}
      />,
    );
    expect(screen.queryByText('(auto)')).not.toBeInTheDocument();
  });

  it('renders ChecksPanel counts and invokes onSelectIssue when clicking a line warning', () => {
    const issues: Issue[] = [
      { severity: 'error', field: 'topic', message: 'topic must be one of ...' },
      { severity: 'warning', line: 12, message: 'Italics are not supported' },
    ];
    const onSelect = vi.fn();
    render(<ChecksPanel issues={issues} onSelectIssue={onSelect} />);

    expect(screen.getByTestId('lint-count')).toHaveTextContent('1 error · 1 warning');
    fireEvent.click(screen.getByText('Italics are not supported'));
    expect(onSelect).toHaveBeenCalledWith(issues[1]);
  });

  it('disables publish confirmation when plan has blocking errors and enables when clean', () => {
    const basePlan: PublishPlan = {
      key: 'my-post',
      path: 'content/posts/2026-11-04-my-post.md',
      publishBranch: 'main',
      unsaved: false,
      issues: [{ severity: 'error', field: 'title', message: 'title is required' }],
      blocking: true,
      files: ['content/posts/2026-11-04-my-post.md'],
      diff: '+title: ""\n',
      steps: ['git fetch origin main', 'git push origin HEAD:main'],
    };

    const onConfirm = vi.fn();
    const { rerender } = render(
      <PublishDialog
        open={true}
        title="My Post"
        slug="my-post"
        remote="origin"
        publishBranch="main"
        plan={basePlan}
        loadingPlan={false}
        publishing={false}
        activeStep={null}
        completedSteps={[]}
        result={null}
        error={null}
        onClose={() => {}}
        onConfirm={onConfirm}
      />,
    );

    const pushBtn = screen.getByRole('button', { name: 'Push to main' });
    expect(pushBtn).toBeDisabled();

    rerender(
      <PublishDialog
        open={true}
        title="My Post"
        slug="my-post"
        remote="origin"
        publishBranch="main"
        plan={{ ...basePlan, issues: [], blocking: false }}
        loadingPlan={false}
        publishing={false}
        activeStep={null}
        completedSteps={[]}
        result={null}
        error={null}
        onClose={() => {}}
        onConfirm={onConfirm}
      />,
    );

    expect(pushBtn).not.toBeDisabled();
    fireEvent.click(pushBtn);
    expect(onConfirm).toHaveBeenCalledTimes(1);
  });

  it('renders Preview using the production MarkdownDispatcher component', () => {
    const fm: Frontmatter = {
      title: 'Production Renderer Check',
      slug: 'prod-check',
      date: '2026-11-04',
      topic: 'cloud-native',
      project: 'nacfson_blog',
      summary: 'Deck summary text.',
      tags: ['k3s'],
    };
    const md = [
      '## Architecture Overview',
      '',
      '```benchmarkTable',
      JSON.stringify({
        title: 'Comparison Matrix',
        columns: ['Metric', 'Studio'],
        highlightColumn: 1,
        rows: [{ metric: 'Renderer', values: ['**Shared**'] }],
      }),
      '```',
    ].join('\n');

    render(<Preview frontmatter={fm} markdown={md} />);
    expect(screen.getByText('Production Renderer Check')).toBeInTheDocument();
    expect(screen.getByText('Cloud-Native & Kubernetes')).toBeInTheDocument();
    expect(screen.getByText('Architecture Overview')).toBeInTheDocument();
    expect(screen.getByText('Comparison Matrix')).toBeInTheDocument();
    expect(screen.getByText('Shared')).toBeInTheDocument();
  });
});
