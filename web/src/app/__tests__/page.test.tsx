import React from 'react';
import { fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import '@testing-library/jest-dom/vitest';
import { afterEach, describe, expect, it, vi } from 'vitest';
import HomePage from '../page';

type P = { slug: string; title: string; date: string; topic: string; summary: string; tags: string[]; readTimeMinutes: number };

function post(i: number, topic = 'cloud-native'): P {
  return {
    slug: `post-${i}`,
    title: `Post Title ${i}`,
    date: `2026-10-${String(30 - i).padStart(2, '0')}`,
    topic,
    summary: `Summary ${i}`,
    tags: ['tag'],
    readTimeMinutes: 3,
  };
}

function mockFetch(posts: P[], ok = true) {
  const fetchMock = vi.fn().mockResolvedValue({
    ok,
    status: ok ? 200 : 500,
    json: () => Promise.resolve(posts),
  });
  vi.stubGlobal('fetch', fetchMock);
  return fetchMock;
}

afterEach(() => {
  vi.unstubAllGlobals();
  window.history.replaceState(null, '', '/');
});

describe('HomePage', () => {
  it('renders the Make Better World hero and the four topics in the rail', async () => {
    mockFetch([]);
    render(<HomePage />);

    expect(screen.getByRole('heading', { level: 1, name: 'Make Better World' })).toBeInTheDocument();
    const rail = screen.getByRole('navigation', { name: 'Topics' });
    for (const label of ['All articles', 'Cloud-Native & Kubernetes', 'Zero-Trust Security', 'AI & LLM Agents', 'Open Source & Community']) {
      expect(within(rail).getByRole('button', { name: new RegExp(label.replace(/[&]/g, '\\&')) })).toBeInTheDocument();
    }
    await waitFor(() => expect(screen.getByRole('status')).toBeInTheDocument());
  });

  it('shows a friendly "blogger is writing" notice when there are no posts', async () => {
    mockFetch([]);
    render(<HomePage />);

    const notice = await screen.findByRole('status');
    expect(within(notice).getByText(/The blogger is writing the first story/i)).toBeInTheDocument();
    expect(within(notice).getByRole('link', { name: /Follow on GitHub/i })).toHaveAttribute('href', 'https://github.com/nacfson');
    expect(screen.queryByRole('region', { name: 'Featured articles' })).not.toBeInTheDocument();
    expect(screen.queryByRole('list', { name: 'Articles' })).not.toBeInTheDocument();
  });

  it('shows the same notice when the posts API fails', async () => {
    vi.spyOn(console, 'warn').mockImplementation(() => {});
    mockFetch([], false);
    render(<HomePage />);
    expect(await screen.findByText(/The blogger is writing the first story/i)).toBeInTheDocument();
  });

  it('renders featured shelf and grid, and filters by topic with a topic-level empty state', async () => {
    const fetchMock = mockFetch([post(1, 'cloud-native'), post(2, 'zero-trust')]);
    render(<HomePage />);

    const grid = await screen.findByRole('list', { name: 'Articles' });
    expect(within(grid).getAllByRole('listitem')).toHaveLength(2);
    expect(screen.getByRole('region', { name: 'Featured articles' })).toBeInTheDocument();
    expect(screen.getByText('Showing 2 of 2 articles')).toBeInTheDocument();
    expect(screen.queryByText(/Competency Matrix/i)).not.toBeInTheDocument();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledWith('/api/posts');

    const rail = screen.getByRole('navigation', { name: 'Topics' });
    fireEvent.click(within(rail).getByRole('button', { name: /Zero-Trust Security/ }));
    expect(within(screen.getByRole('list', { name: 'Articles' })).getAllByRole('listitem')).toHaveLength(1);
    expect(screen.getByText('Showing 1 of 2 articles')).toBeInTheDocument();
    expect(window.location.search).toBe('?topic=zero-trust');

    fireEvent.click(within(rail).getByRole('button', { name: /Open Source & Community/ }));
    expect(screen.getByText(/Nothing in Open Source & Community yet/i)).toBeInTheDocument();
    fireEvent.click(screen.getByRole('button', { name: /Show all articles/i }));
    expect(within(screen.getByRole('list', { name: 'Articles' })).getAllByRole('listitem')).toHaveLength(2);
    expect(window.location.search).toBe('');
  });

  it('selects the topic from the ?topic= query string on load', async () => {
    window.history.replaceState(null, '', '/?topic=zero-trust');
    mockFetch([post(1, 'cloud-native'), post(2, 'zero-trust')]);
    render(<HomePage />);

    const grid = await screen.findByRole('list', { name: 'Articles' });
    expect(within(grid).getAllByRole('listitem')).toHaveLength(1);
    expect(within(grid).getByText('Post Title 2')).toBeInTheDocument();
  });

  it('paginates the grid with View more', async () => {
    mockFetch(Array.from({ length: 11 }, (_, i) => post(i + 1)));
    render(<HomePage />);

    const grid = await screen.findByRole('list', { name: 'Articles' });
    expect(within(grid).getAllByRole('listitem')).toHaveLength(9);
    fireEvent.click(screen.getByRole('button', { name: 'View more' }));
    expect(within(screen.getByRole('list', { name: 'Articles' })).getAllByRole('listitem')).toHaveLength(11);
    expect(screen.queryByRole('button', { name: 'View more' })).not.toBeInTheDocument();
  });
});
