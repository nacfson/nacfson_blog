'use client';

import React, { useState, useEffect } from 'react';
import Link from 'next/link';
import { PostSummary, CompetencyMatrix, Pillar } from '../lib/types';

const PILLARS: { id: Pillar | 'all'; label: string; description: string }[] = [
  { id: 'all', label: 'All Articles', description: 'Complete engineering log across all pillars' },
  { id: 'project-updates', label: 'Project Architecture', description: 'Milestones & internals of nacfson_pipeline, Project_PN, and tenants' },
  { id: 'competency-growth', label: 'Competency Growth', description: 'Structured systems, CNCF, and zero-trust engineering milestones' },
  { id: 'tech-opinions', label: 'Technical Opinions', description: 'Deep dives, interactive benchmarks, and architecture tradeoffs' },
];

export default function HomePage() {
  const [selectedPillar, setSelectedPillar] = useState<Pillar | 'all'>('all');
  const [posts, setPosts] = useState<PostSummary[]>([]);
  const [loadingPosts, setLoadingPosts] = useState(true);
  const [competencies, setCompetencies] = useState<CompetencyMatrix | null>(null);

  // Fetch posts when selectedPillar changes
  useEffect(() => {
    setLoadingPosts(true);
    const query = selectedPillar !== 'all' ? `?pillar=${selectedPillar}` : '';
    fetch(`/api/posts${query}`)
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: PostSummary[]) => {
        setPosts(data || []);
      })
      .catch((err) => {
        console.warn('Failed to load posts from /api/posts:', err);
        setPosts([]);
      })
      .finally(() => setLoadingPosts(false));
  }, [selectedPillar]);

  // Fetch competency matrix once
  useEffect(() => {
    fetch('/api/competencies')
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`);
        return res.json();
      })
      .then((data: CompetencyMatrix) => {
        setCompetencies(data);
      })
      .catch((err) => {
        console.warn('Failed to load competencies from /api/competencies:', err);
      });
  }, []);

  return (
    <div className="mx-auto max-w-7xl px-4 sm:px-6 lg:px-8 py-10 space-y-16">
      {/* Portfolio Header */}
      <section className="relative overflow-hidden rounded-3xl border border-slate-200/80 bg-white p-8 sm:p-12 shadow-sm">
        <div className="max-w-3xl space-y-4">
          <div className="inline-flex items-center gap-2 rounded-full border border-blue-200/80 bg-blue-50/60 px-3 py-1 text-xs font-medium text-blue-700">
            <span className="h-1.5 w-1.5 rounded-full bg-blue-600" />
            Interactive Engineering Log
          </div>
          <h1 className="text-3xl sm:text-4xl lg:text-5xl font-extrabold tracking-tight text-slate-900 leading-tight">
            Systems, Cloud-Native &amp; Zero-Trust Architecture
          </h1>
          <p className="text-sm sm:text-base text-slate-600 leading-relaxed">
            Welcome to my technical portfolio and engineering notebook. Here you will find deep architectural walkthroughs, live interactive simulators, and verifiable competency tracking built for single-node to multi-tenant production Kubernetes.
          </p>
        </div>
      </section>

      {/* Content Pillars Filter */}
      <section className="space-y-6">
        <div className="flex flex-col sm:flex-row sm:items-center justify-between gap-4 border-b border-slate-200 pb-4">
          <div>
            <h2 className="text-xl font-bold tracking-tight text-slate-900">Articles &amp; Architecture Notes</h2>
            <p className="text-xs text-slate-500 mt-0.5">Filter by engineering focus area</p>
          </div>

          <div className="flex flex-wrap gap-2">
            {PILLARS.map((p) => {
              const isActive = selectedPillar === p.id;
              return (
                <button
                  key={p.id}
                  type="button"
                  onClick={() => setSelectedPillar(p.id)}
                  className={`px-3.5 py-1.5 rounded-xl text-xs font-medium transition-all ${
                    isActive
                      ? 'bg-blue-600 text-white shadow-sm shadow-blue-500/20'
                      : 'bg-white border border-slate-200 text-slate-600 hover:bg-slate-50 hover:text-slate-900'
                  }`}
                >
                  {p.label}
                </button>
              );
            })}
          </div>
        </div>

        {/* Post Feed */}
        {loadingPosts ? (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {[1, 2, 3].map((i) => (
              <div key={i} className="h-64 rounded-2xl border border-slate-200 bg-white p-6 animate-pulse" />
            ))}
          </div>
        ) : posts.length === 0 ? (
          <div className="rounded-2xl border border-dashed border-slate-300 bg-white p-12 text-center">
            <p className="text-sm font-medium text-slate-600">No articles found for this pillar yet.</p>
            <p className="text-xs text-slate-400 mt-1">Check back soon as new posts are synced live via FluxCD.</p>
          </div>
        ) : (
          <div className="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-6">
            {posts.map((post) => (
              <Link
                key={post.slug}
                href={`/post?slug=${post.slug}`}
                className="group flex flex-col justify-between rounded-2xl border border-slate-200/90 bg-white p-6 shadow-sm hover:shadow-md hover:border-blue-300 transition-all"
              >
                <div className="space-y-3">
                  <div className="flex items-center justify-between text-[11px] text-slate-500">
                    <span className="font-mono">{post.date}</span>
                    <span className="font-medium text-slate-400">{post.readTimeMinutes} min read</span>
                  </div>

                  <h3 className="text-base font-bold text-slate-900 group-hover:text-blue-600 transition-colors line-clamp-2">
                    {post.title}
                  </h3>

                  <p className="text-xs text-slate-600 leading-relaxed line-clamp-3">
                    {post.summary}
                  </p>
                </div>

                <div className="mt-6 pt-4 border-t border-slate-100 flex flex-wrap gap-1.5">
                  {post.tags.slice(0, 3).map((tag) => (
                    <span
                      key={tag}
                      className="px-2 py-0.5 rounded-md text-[10px] font-medium bg-slate-100 text-slate-600 group-hover:bg-blue-50 group-hover:text-blue-700 transition-colors"
                    >
                      #{tag}
                    </span>
                  ))}
                  {post.tags.length > 3 && (
                    <span className="px-1.5 py-0.5 rounded-md text-[10px] font-medium text-slate-400">
                      +{post.tags.length - 3}
                    </span>
                  )}
                </div>
              </Link>
            ))}
          </div>
        )}
      </section>

      {/* Competency Matrix Section */}
      <section id="competencies" className="space-y-6 pt-6">
        <div className="border-b border-slate-200 pb-4">
          <div className="inline-flex items-center gap-1.5 text-xs font-semibold uppercase tracking-wider text-blue-600 mb-1">
            <span>Verified Skills</span>
          </div>
          <h2 className="text-2xl font-bold tracking-tight text-slate-900">Technical Competency Matrix</h2>
          <p className="text-xs text-slate-500 mt-1">
            Proven platform and systems capabilities backed by production code and interactive architecture articles.
          </p>
        </div>

        {competencies ? (
          <div className="grid grid-cols-1 lg:grid-cols-3 gap-6">
            {competencies.domains.map((domain) => (
              <div
                key={domain.id}
                className="rounded-2xl border border-slate-200 bg-white p-6 shadow-sm flex flex-col justify-between"
              >
                <div className="space-y-4">
                  <div className="flex items-center justify-between border-b border-slate-100 pb-3">
                    <span className="text-xs font-bold text-slate-900">{domain.name}</span>
                    <span className="px-2 py-0.5 rounded-full text-[10px] font-semibold bg-blue-50 text-blue-700 border border-blue-200/60">
                      {domain.level}
                    </span>
                  </div>

                  <p className="text-xs text-slate-600">{domain.description}</p>

                  <div className="space-y-3 pt-2">
                    {domain.competencies.map((comp) => (
                      <div key={comp.name} className="rounded-xl border border-slate-100 bg-slate-50/60 p-3 space-y-2">
                        <div className="flex items-center justify-between">
                          <span className="text-xs font-semibold text-slate-800">{comp.name}</span>
                          <span className="text-[10px] font-medium text-emerald-600 bg-emerald-50 px-1.5 py-0.5 rounded border border-emerald-200/60">
                            {comp.status}
                          </span>
                        </div>

                        <div className="flex flex-wrap gap-1">
                          {comp.skills.map((s) => (
                            <span key={s} className="px-1.5 py-0.5 rounded text-[10px] bg-white border border-slate-200 text-slate-600 font-mono">
                              {s}
                            </span>
                          ))}
                        </div>

                        {comp.linkedPosts && comp.linkedPosts.length > 0 && (
                          <div className="pt-1 flex items-center gap-1.5 text-[11px] text-blue-600">
                            <span>Article:</span>
                            {comp.linkedPosts.map((slug) => (
                              <Link
                                key={slug}
                                href={`/post?slug=${slug}`}
                                className="underline hover:text-blue-800 font-medium"
                              >
                                {slug}
                              </Link>
                            ))}
                          </div>
                        )}
                      </div>
                    ))}
                  </div>
                </div>
              </div>
            ))}
          </div>
        ) : (
          <div className="rounded-2xl border border-slate-200 bg-white p-8 text-center text-xs text-slate-400 animate-pulse">
            Loading Competency Matrix...
          </div>
        )}
      </section>
    </div>
  );
}
