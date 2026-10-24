'use client';

import React, { useState, useEffect, Suspense } from 'react';
import { useSearchParams } from 'next/navigation';
import Link from 'next/link';
import { Post } from '../../lib/types';
import { BlogV2MiniNav } from '../../components/BlogV2MiniNav';
import { MarkdownDispatcher } from '../../components/MarkdownDispatcher';

function PostContent() {
  const searchParams = useSearchParams();
  const searchSlug = searchParams.get('slug');

  const [post, setPost] = useState<Post | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let slug = searchSlug;
    if (!slug && typeof window !== 'undefined') {
      const match = window.location.pathname.match(/\/post\/([^/?#]+)/);
      if (match && match[1]) {
        slug = decodeURIComponent(match[1]);
      }
    }

    if (!slug) {
      setLoading(false);
      setError('No post slug specified in query string or URL path.');
      return;
    }

    setLoading(true);
    setError(null);

    let active = true;

    fetch(`/api/posts/${encodeURIComponent(slug)}`)
      .then((res) => {
        if (!res.ok) {
          if (res.status === 404) throw new Error('Post not found');
          throw new Error(`Failed to load post (HTTP ${res.status})`);
        }
        return res.json();
      })
      .then((data: Post) => {
        if (active) {
          setPost(data);
        }
      })
      .catch((err: Error) => {
        if (active) {
          setError(err.message);
        }
      })
      .finally(() => {
        if (active) {
          setLoading(false);
        }
      });

    return () => {
      active = false;
    };
  }, [searchSlug]);

  if (loading) {
    return (
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-12">
        <div className="space-y-4 max-w-4xl animate-pulse">
          <div className="h-4 w-24 bg-slate-200 rounded" />
          <div className="h-10 w-3/4 bg-slate-200 rounded-lg" />
          <div className="h-4 w-1/2 bg-slate-200 rounded" />
          <div className="h-72 w-full bg-slate-200 rounded-2xl mt-8" />
        </div>
      </div>
    );
  }

  if (error || !post) {
    return (
      <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-16">
        <div className="max-w-xl mx-auto text-center space-y-4">
          <div className="inline-flex p-3 rounded-full bg-red-100 text-red-600">
            <svg className="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M12 9v2m0 4h.01m-6.938 4h13.856c1.54 0 2.502-1.667 1.732-3L13.732 4c-.77-1.333-2.694-1.333-3.464 0L3.34 16c-.77 1.333.192 3 1.732 3z" />
            </svg>
          </div>
          <h1 className="text-xl font-bold text-slate-900">Article Not Found</h1>
          <p className="text-xs text-slate-500">{error || 'The requested article could not be loaded.'}</p>
          <div className="pt-2">
            <Link
              href="/"
              className="inline-flex items-center gap-1.5 px-4 py-2 rounded-xl text-xs font-semibold bg-blue-600 text-white hover:bg-blue-700 transition-colors shadow-sm"
            >
              &larr; Return to Home
            </Link>
          </div>
        </div>
      </div>
    );
  }

  return (
    <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-8 sm:py-12">
      {/* Back button & Meta */}
      <div className="mb-6 flex items-center justify-between">
        <Link
          href="/"
          className="inline-flex items-center gap-1.5 text-xs font-medium text-slate-500 hover:text-blue-600 transition-colors"
        >
          <svg className="w-3.5 h-3.5" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path strokeLinecap="round" strokeLinejoin="round" strokeWidth={2} d="M15 19l-7-7 7-7" />
          </svg>
          Back to Articles
        </Link>

        <div className="flex items-center gap-2">
          <span className="px-2.5 py-0.5 rounded-full text-[10px] font-semibold bg-blue-50 text-blue-700 border border-blue-200">
            {post.pillar}
          </span>
          {post.project && (
            <span className="px-2.5 py-0.5 rounded-full text-[10px] font-medium bg-slate-100 text-slate-600">
              {post.project}
            </span>
          )}
        </div>
      </div>

      {/* Article Header */}
      <header className="mb-8 max-w-4xl space-y-3">
        <div className="flex items-center gap-3 text-xs text-slate-400 font-mono">
          <time dateTime={post.date}>{post.date}</time>
          <span>&middot;</span>
          <span>{post.readTimeMinutes} min read</span>
        </div>

        <h1 className="text-2xl sm:text-3xl lg:text-4xl font-extrabold tracking-tight text-slate-900 leading-tight">
          {post.title}
        </h1>

        <p className="text-sm sm:text-base text-slate-600 leading-relaxed pt-1">
          {post.summary}
        </p>

        <div className="flex flex-wrap gap-1.5 pt-2">
          {post.tags.map((tag) => (
            <span
              key={tag}
              className="px-2 py-0.5 rounded text-[10px] font-medium bg-slate-100 text-slate-600 font-mono"
            >
              #{tag}
            </span>
          ))}
        </div>
      </header>

      {/* Two Column Layout: MiniNav + Markdown Content */}
      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8 items-start">
        {/* Left Sticky TOC */}
        <aside className="lg:col-span-3">
          <BlogV2MiniNav />
        </aside>

        {/* Right Article Body */}
        <article className="lg:col-span-9 max-w-none">
          <MarkdownDispatcher markdown={post.markdownBody} />
        </article>
      </div>
    </div>
  );
}

export default function PostPage() {
  return (
    <Suspense
      fallback={
        <div className="mx-auto max-w-7xl px-4 py-16 text-center text-xs text-slate-400 animate-pulse">
          Loading article...
        </div>
      }
    >
      <PostContent />
    </Suspense>
  );
}
