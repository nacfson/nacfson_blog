'use client';

import React, { useEffect, useState } from 'react';

export interface MiniNavHeading {
  id: string;
  text: string;
}

interface BlogV2MiniNavProps {
  headings?: MiniNavHeading[];
}

export const BlogV2MiniNav: React.FC<BlogV2MiniNavProps> = ({ headings: initialHeadings }) => {
  const [headings, setHeadings] = useState<MiniNavHeading[]>(initialHeadings || []);
  const [activeId, setActiveId] = useState<string>('');

  // If headings are not provided, scan the DOM for h2 tags inside article
  useEffect(() => {
    if (initialHeadings && initialHeadings.length > 0) {
      setHeadings(initialHeadings);
      return;
    }

    const elements = Array.from(document.querySelectorAll('article h2, main h2'));
    const detected: MiniNavHeading[] = elements.map((el, i) => {
      let id = el.id;
      if (!id) {
        id = el.textContent?.trim().toLowerCase().replace(/[^a-z0-9]+/g, '-') || `section-${i}`;
        el.id = id;
      }
      return {
        id,
        text: el.textContent?.trim() || `Section ${i + 1}`,
      };
    });

    setHeadings(detected);
  }, [initialHeadings]);

  // Scroll spy to highlight active heading
  useEffect(() => {
    if (headings.length === 0) return;

    const observer = new IntersectionObserver(
      (entries) => {
        entries.forEach((entry) => {
          if (entry.isIntersecting) {
            setActiveId(entry.target.id);
          }
        });
      },
      {
        rootMargin: '0px 0px -70% 0px',
        threshold: 0,
      }
    );

    headings.forEach((h) => {
      const el = document.getElementById(h.id);
      if (el) observer.observe(el);
    });

    return () => observer.disconnect();
  }, [headings]);

  if (headings.length === 0) {
    return null;
  }

  const handleClick = (e: React.MouseEvent<HTMLAnchorElement>, id: string) => {
    e.preventDefault();
    const el = document.getElementById(id);
    if (el) {
      el.scrollIntoView({ behavior: 'smooth', block: 'start' });
      setActiveId(id);
      window.history.pushState(null, '', `#${id}`);
    }
  };

  return (
    <nav
      aria-label="Table of contents"
      className="sticky top-24 max-h-[calc(100vh-8rem)] overflow-y-auto pr-4 text-sm hidden lg:block"
    >
      <div className="text-xs font-semibold uppercase tracking-wider text-slate-400 mb-3 px-3">
        On this page
      </div>
      <ul className="space-y-1 border-l border-slate-200">
        {headings.map((heading) => {
          const isActive = activeId === heading.id;
          return (
            <li key={heading.id}>
              <a
                href={`#${heading.id}`}
                onClick={(e) => handleClick(e, heading.id)}
                className={`group flex items-center -ml-px pl-3 py-1.5 border-l-2 transition-colors text-xs ${
                  isActive
                    ? 'border-blue-600 font-medium text-blue-600 bg-blue-50/50 rounded-r'
                    : 'border-transparent text-slate-600 hover:text-slate-900 hover:border-slate-300'
                }`}
              >
                <span className="line-clamp-2">{heading.text}</span>
              </a>
            </li>
          );
        })}
      </ul>
    </nav>
  );
};
