'use client';

import React, { useEffect, useMemo, useState } from 'react';
import Link from 'next/link';
import { PostSummary, Topic } from '../lib/types';
import { TOPICS, isTopic, topicLabel } from '../lib/topics';

const PAGE_SIZE = 9;
const FEATURED_COUNT = 4;
const GITHUB_URL = 'https://github.com/nacfson';

/** Diagonal hatch used instead of illustrations (monochrome editorial plates). */
const HATCH: React.CSSProperties = {
  backgroundColor: '#f5f5f4',
  backgroundImage: 'repeating-linear-gradient(45deg, #a8a29e 0 1px, transparent 1px 9px)',
};

type TopicFilter = Topic | 'all';

function formatDate(date: string): string {
  const d = new Date(`${date}T00:00:00Z`);
  if (Number.isNaN(d.getTime())) return date;
  return d.toLocaleDateString('en-US', { month: 'short', day: 'numeric', year: 'numeric', timeZone: 'UTC' });
}

export default function HomePage() {
  const [posts, setPosts] = useState<PostSummary[]>([]);
  const [loadingPosts, setLoadingPosts] = useState(true);
  const [topic, setTopic] = useState<TopicFilter>('all');
  const [visible, setVisible] = useState(PAGE_SIZE);

  // Initial topic from ?topic= (client-only: static export has no request-time params).
  useEffect(() => {
    const fromUrl = new URLSearchParams(window.location.search).get('topic');
    if (isTopic(fromUrl)) setTopic(fromUrl);
  }, []);

  useEffect(() => {
    let active = true;
    fetch('/api/posts')
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: PostSummary[]) => {
        if (active) setPosts(data || []);
      })
      .catch((err) => {
        if (active) {
          console.warn('Failed to load posts from /api/posts:', err);
          setPosts([]);
        }
      })
      .finally(() => {
        if (active) setLoadingPosts(false);
      });
    return () => {
      active = false;
    };
  }, []);

  const selectTopic = (next: TopicFilter) => {
    setTopic(next);
    setVisible(PAGE_SIZE);
    const url = next === 'all' ? window.location.pathname : `${window.location.pathname}?topic=${next}`;
    window.history.replaceState(null, '', url);
  };

  const counts = useMemo(() => {
    const c: Record<string, number> = { all: posts.length };
    for (const t of TOPICS) c[t.id] = posts.filter((p) => p.topic === t.id).length;
    return c;
  }, [posts]);

  const filtered = topic === 'all' ? posts : posts.filter((p) => p.topic === topic);
  const featured = posts.slice(0, FEATURED_COUNT);

  return (
    <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-10 grid grid-cols-1 lg:grid-cols-[240px_minmax(0,1fr)] gap-10 lg:gap-12">
      {/* Left rail */}
      <aside className="order-2 lg:order-1 lg:sticky lg:top-28 self-start space-y-7">
        <nav aria-label="Topics">
          <h2 className="mb-2.5 text-[11px] font-semibold uppercase tracking-[0.08em] text-stone-400">Topics</h2>
          <ul className="space-y-0.5">
            {[{ id: 'all' as TopicFilter, label: 'All articles' }, ...TOPICS].map((t) => {
              const isActive = topic === t.id;
              return (
                <li key={t.id}>
                  <button
                    type="button"
                    aria-pressed={isActive}
                    onClick={() => selectTopic(t.id)}
                    className={`flex w-full items-center justify-between rounded px-3 py-2 text-left text-[13px] transition-colors ${
                      isActive
                        ? 'bg-stone-950 font-semibold text-stone-50'
                        : 'text-stone-600 hover:bg-stone-100 hover:text-stone-950'
                    }`}
                  >
                    <span>{t.label}</span>
                    <span className={`text-[11px] ${isActive ? 'text-stone-300' : 'text-stone-400'}`}>{counts[t.id] ?? 0}</span>
                  </button>
                </li>
              );
            })}
          </ul>
        </nav>

        <div className="rounded border border-stone-200 bg-white p-5">
          <p className="text-[11px] font-semibold uppercase tracking-[0.08em] text-stone-500">About the blogger</p>
          <p className="mt-1.5 font-serif text-lg font-bold text-stone-950">nacfson</p>
          <p className="mt-2 text-xs leading-relaxed text-stone-600">
            I believe good engineering can make the world a little better every day. Notes from building secure, open,
            and intelligent systems.
          </p>
          <a
            href={GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            className="mt-3 inline-block text-xs font-semibold text-stone-950 underline underline-offset-4 hover:no-underline"
          >
            GitHub profile →
          </a>
        </div>
      </aside>

      {/* Main column */}
      <div className="order-1 lg:order-2 min-w-0">
        <section>
          <h1 className="font-serif text-4xl sm:text-5xl font-bold tracking-tight text-stone-950">Make Better World</h1>
          <p className="mt-3 max-w-2xl text-base leading-relaxed text-stone-600">
            I believe good engineering can make the world a little better every day. Here I share what I learn while
            building cloud-native platforms, zero-trust security, AI agents, and the open-source communities behind them.
          </p>
        </section>

        {loadingPosts ? (
          <div className="mt-10 space-y-6" aria-busy="true">
            <div className="h-64 rounded border border-stone-200 bg-white animate-pulse" />
            <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-5">
              {[1, 2, 3].map((i) => (
                <div key={i} className="h-72 rounded border border-stone-200 bg-white animate-pulse" />
              ))}
            </div>
          </div>
        ) : posts.length === 0 ? (
          <EmptyNotice />
        ) : (
          <>
            <FeaturedShelf posts={featured} />

            <div className="mt-12 flex items-center justify-between border-b border-stone-200 pb-3.5 text-xs text-stone-500">
              <span>{`Showing ${Math.min(visible, filtered.length)} of ${posts.length} articles`}</span>
              <span className="rounded border border-stone-200 bg-white px-3 py-1.5 text-stone-600">Newest first</span>
            </div>

            {filtered.length === 0 ? (
              <div className="mt-6 rounded border border-dashed border-stone-300 bg-white p-10 text-center">
                <p className="font-serif text-lg font-bold text-stone-950">{`Nothing in ${topicLabel(topic)} yet.`}</p>
                <p className="mt-1 text-xs text-stone-500">The blogger hasn&apos;t written about this topic so far.</p>
                <button
                  type="button"
                  onClick={() => selectTopic('all')}
                  className="mt-5 rounded border border-stone-950 px-4 py-2 text-xs font-semibold text-stone-950 hover:bg-stone-950 hover:text-stone-50 transition-colors"
                >
                  Show all articles
                </button>
              </div>
            ) : (
              <ul aria-label="Articles" className="mt-6 grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-5">
                {filtered.slice(0, visible).map((post) => (
                  <li key={post.slug}>
                    <PostCard post={post} />
                  </li>
                ))}
              </ul>
            )}

            {filtered.length > visible && (
              <div className="mt-8 flex justify-center">
                <button
                  type="button"
                  onClick={() => setVisible((v) => v + PAGE_SIZE)}
                  className="rounded bg-stone-950 px-6 py-2.5 text-[13px] font-semibold text-stone-50 hover:bg-stone-800 transition-colors"
                >
                  View more
                </button>
              </div>
            )}
          </>
        )}
      </div>
    </div>
  );
}

function PostCard({ post }: { post: PostSummary }) {
  return (
    <Link
      href={`/post?slug=${post.slug}`}
      className="group flex h-full flex-col overflow-hidden rounded border border-stone-200 bg-white transition-all hover:-translate-y-0.5 hover:border-stone-950"
    >
      <div className="h-28 border-b border-stone-200" style={HATCH} aria-hidden="true" />
      <div className="flex flex-1 flex-col gap-2 p-5">
        <div className="flex items-center justify-between text-[11px]">
          <span className="font-semibold uppercase tracking-[0.06em] text-stone-950">{topicLabel(post.topic, 'short')}</span>
          <span className="text-stone-400">{formatDate(post.date)}</span>
        </div>
        <h3 className="font-serif text-[17px] font-bold leading-snug text-stone-950 group-hover:underline underline-offset-4 line-clamp-3">
          {post.title}
        </h3>
        <p className="text-xs leading-relaxed text-stone-600 line-clamp-3">{post.summary}</p>
        <div className="mt-auto flex flex-wrap gap-1.5 pt-3">
          {post.tags.slice(0, 3).map((tag) => (
            <span key={tag} className="rounded-sm bg-stone-100 px-2 py-0.5 text-[10px] font-medium text-stone-600">
              #{tag}
            </span>
          ))}
          {post.tags.length > 3 && <span className="px-1 py-0.5 text-[10px] text-stone-400">+{post.tags.length - 3}</span>}
        </div>
      </div>
    </Link>
  );
}

function FeaturedShelf({ posts }: { posts: PostSummary[] }) {
  const [open, setOpen] = useState(0);
  if (posts.length === 0) return null;
  const current = posts[Math.min(open, posts.length - 1)];

  return (
    <section aria-label="Featured articles" className="mt-10">
      <div className="flex h-auto gap-2.5 md:h-72">
        <Link
          href={`/post?slug=${current.slug}`}
          className="group grid min-w-0 flex-1 grid-cols-1 overflow-hidden rounded border border-stone-200 bg-white hover:border-stone-950 transition-colors md:grid-cols-[1.1fr_1fr]"
        >
          <div className="flex flex-col justify-between gap-6 p-7">
            <div>
              <p className="text-[11px] text-stone-400">
                <span className="mr-2 font-semibold uppercase tracking-[0.06em] text-stone-950">
                  {topicLabel(current.topic, 'short')}
                </span>
                {formatDate(current.date)}
              </p>
              <h2 className="mt-2.5 font-serif text-2xl font-bold leading-tight text-stone-950 line-clamp-3">{current.title}</h2>
              <p className="mt-2.5 text-[13px] leading-relaxed text-stone-600 line-clamp-3">{current.summary}</p>
            </div>
            <span className="text-xs font-semibold text-stone-950 underline underline-offset-4 group-hover:no-underline">
              Read article →
            </span>
          </div>
          <div className="hidden border-l border-stone-200 md:block" style={HATCH} aria-hidden="true" />
        </Link>

        {posts.length > 1 &&
          posts.map((p, i) =>
            i === open ? null : (
              <button
                key={p.slug}
                type="button"
                onClick={() => setOpen(i)}
                aria-label={`Open featured: ${p.title}`}
                className="hidden w-14 flex-none items-center justify-center overflow-hidden rounded border border-stone-200 bg-white hover:border-stone-950 transition-colors md:flex"
              >
                <span className="max-h-60 truncate text-xs font-semibold text-stone-600 [writing-mode:vertical-rl] rotate-180">
                  {p.title}
                </span>
              </button>
            ),
          )}
      </div>
      {posts.length > 1 && <p className="mt-2 hidden text-[11px] text-stone-400 md:block">Featured: click a spine to open it</p>}
    </section>
  );
}

function EmptyNotice() {
  return (
    <div role="status" aria-live="polite" className="mt-10 overflow-hidden rounded border border-stone-200 bg-white">
      <div className="flex items-center gap-2 border-b border-stone-200 bg-stone-100 px-5 py-2.5 text-[11px] text-stone-500">
        <span className="relative flex h-2 w-2" aria-hidden="true">
          <span className="absolute inline-flex h-full w-full animate-ping rounded-full bg-stone-950 opacity-40" />
          <span className="relative inline-flex h-2 w-2 rounded-full bg-stone-950" />
        </span>
        <span className="font-semibold uppercase tracking-[0.08em] text-stone-950">Notice</span>
        <span aria-hidden="true">·</span>
        <span>just now</span>
      </div>

      <div className="flex flex-col gap-6 p-6 sm:flex-row sm:p-8">
        <div
          className="flex h-12 w-12 flex-none items-center justify-center rounded border border-stone-950 text-stone-950"
          aria-hidden="true"
        >
          <svg className="h-5 w-5" fill="none" viewBox="0 0 24 24" stroke="currentColor" strokeWidth={1.8}>
            <path
              strokeLinecap="round"
              strokeLinejoin="round"
              d="M14.857 17.082a23.848 23.848 0 005.454-1.31A8.967 8.967 0 0118 9.75V9A6 6 0 006 9v.75a8.967 8.967 0 01-2.312 6.022c1.733.64 3.56 1.085 5.455 1.31m5.714 0a24.255 24.255 0 01-5.714 0m5.714 0a3 3 0 11-5.714 0"
            />
          </svg>
        </div>

        <div className="min-w-0">
          <p className="font-serif text-2xl font-bold leading-tight text-stone-950 sm:text-3xl">
            The blogger is writing the first story
            <span className="inline-flex gap-1 pl-1.5 align-middle" aria-hidden="true">
              <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-stone-950 [animation-delay:-0.3s]" />
              <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-stone-950 [animation-delay:-0.15s]" />
              <span className="h-1.5 w-1.5 animate-bounce rounded-full bg-stone-950" />
            </span>
          </p>
          <p className="mt-3 max-w-xl text-sm leading-relaxed text-stone-600">
            Drafts on cloud-native platforms, zero-trust security, AI agents, and open source are on the desk. Check back
            soon, or follow along on GitHub to see what&apos;s being built in the meantime.
          </p>
          <div className="mt-6 flex flex-wrap items-center gap-3">
            <a
              href={GITHUB_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="rounded bg-stone-950 px-4 py-2 text-xs font-semibold text-stone-50 hover:bg-stone-800 transition-colors"
            >
              Follow on GitHub ↗
            </a>
            <span className="rounded border border-stone-300 px-4 py-2 text-xs text-stone-600">First post coming soon</span>
          </div>
        </div>
      </div>

      <div className="h-3 border-t border-stone-200" style={HATCH} aria-hidden="true" />
    </div>
  );
}
