import { DraftState, Frontmatter } from './types';

export type CustomBlockName =
  | 'blogPicture'
  | 'benchmarkChart'
  | 'benchmarkTable'
  | 'blogIframeTabs'
  | 'scrollHero';

export const BLOCK_NAMES: CustomBlockName[] = [
  'blogPicture',
  'benchmarkChart',
  'benchmarkTable',
  'blogIframeTabs',
  'scrollHero',
];

export const BLOCK_TEMPLATES: Record<CustomBlockName, string> = {
  blogPicture: `{\n  "src": "/content/assets/your-figure.svg",\n  "alt": "Describe the image",\n  "caption": "Figure N: …",\n  "aspectRatio": "1200/520"\n}`,
  benchmarkChart: `{\n  "title": "Metric (lower is better)",\n  "unit": "ms",\n  "series": [\n    { "name": "Baseline", "value": 4.1 },\n    { "name": "Ours", "value": 0.6 }\n  ]\n}`,
  benchmarkTable: `{\n  "title": "Comparison",\n  "columns": ["Capability", "Ours", "Default"],\n  "highlightColumn": 1,\n  "rows": [\n    { "metric": "Row", "values": ["**Best**", "Other"] }\n  ]\n}`,
  blogIframeTabs: `{\n  "mountStrategy": "click-or-center",\n  "height": "520px",\n  "tabs": [\n    {\n      "label": "Live demo",\n      "captionTitle": "What it shows",\n      "src": "/content/demos/your-demo.html",\n      "poster": "/content/assets/your-demo-poster.svg"\n    }\n  ]\n}`,
  scrollHero: `{\n  "ariaLabel": "Article hero",\n  "aspectRatio": "1600/760",\n  "baseImage": "/content/assets/hero-bg.svg",\n  "layers": [\n    { "src": "/content/assets/hero-front.svg", "depth": 0.9 }\n  ]\n}`,
};

export function slugify(title: string): string {
  return title
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '-')
    .replace(/^-+|-+$/g, '')
    .slice(0, 64);
}

export function postFilePath(date: string, slug: string): string {
  return `content/posts/${date || 'YYYY-MM-DD'}-${slug || 'untitled'}.md`;
}

export function estimateReadMinutes(body: string): number {
  const words = body.split(/\s+/).filter(Boolean).length;
  return Math.max(1, Math.floor(words / 200));
}

export function serializeFrontmatterPreview(fm: Frontmatter, body: string): string {
  const q = (s: string) => `"${s}"`;
  const lines = [
    '---',
    `title: ${q(fm.title)}`,
    `slug: ${q(fm.slug)}`,
    `date: ${q(fm.date)}`,
    `topic: ${q(fm.topic)}`,
  ];
  if (fm.project) {
    lines.push(`project: ${q(fm.project)}`);
  }
  lines.push(`summary: ${q(fm.summary)}`);
  lines.push(`tags: [${fm.tags.map((t) => q(t.trim())).join(', ')}]`);
  if (fm.readTimeMinutes && fm.readTimeMinutes > 0) {
    lines.push(`readTimeMinutes: ${fm.readTimeMinutes}`);
  }
  if (fm.extra?.length) {
    lines.push(...fm.extra);
  }
  lines.push('---', '', '');
  return lines.join('\n') + body.replace(/\r\n/g, '\n');
}

export function formatHeaderStatus(
  state: DraftState,
  ahead: number,
  publishBranch: string,
  autosavedAt: string | null,
): string {
  switch (state) {
    case 'unsaved':
      return `Unsaved changes${autosavedAt ? ` · autosaved to worktree ${autosavedAt}` : ''}`;
    case 'new':
      return 'New draft · nothing committed yet';
    case 'saved':
      return `Saved locally · ${ahead} commit${ahead === 1 ? '' : 's'} not on ${publishBranch}`;
    case 'live':
      return `Published · in sync with origin/${publishBranch}`;
  }
}

export function dotClassForState(state: DraftState): string {
  switch (state) {
    case 'unsaved':
      return 'dot dirty';
    case 'new':
    case 'saved':
      return 'dot saved';
    case 'live':
      return 'dot pub';
  }
}
