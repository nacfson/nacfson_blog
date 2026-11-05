import { Topic, PostSummary } from '@web/lib/types';

export type DraftState = 'unsaved' | 'new' | 'saved' | 'live';

export interface DraftStatus {
  key: string;
  branch: string;
  worktree: string;
  path: string;
  state: DraftState;
  ahead: number;
}

export interface DraftView extends DraftStatus {
  title: string;
  slug: string;
  date: string;
}

export interface PublishedPost {
  path: string;
  summary: PostSummary;
  error?: string;
}

export interface StateResponse {
  publishBranch: string;
  remote: string;
  repo: string;
  active: string;
  drafts: DraftView[];
  published: PublishedPost[];
}

export interface Frontmatter {
  title: string;
  slug: string;
  date: string;
  topic: Topic | '';
  project: string;
  summary: string;
  tags: string[];
  readTimeMinutes?: number;
  extra?: string[];
}

export interface Document {
  frontmatter: Frontmatter;
  body: string;
}

export interface Issue {
  severity: 'error' | 'warning';
  message: string;
  field?: string;
  line?: number;
}

export interface DraftResponse {
  draft: DraftStatus;
  document: Document;
  issues: Issue[];
}

export interface SaveResponse {
  commit?: string;
  path?: string;
  nothingToCommit?: boolean;
}

export interface PublishPlan {
  key: string;
  path: string;
  publishBranch: string;
  unsaved: boolean;
  issues: Issue[];
  blocking: boolean;
  files: string[];
  diff: string;
  steps: string[];
}

export interface PublishResult {
  commit: string;
  publishBranch: string;
  path: string;
  slug: string;
}

export interface AssetResult {
  url: string;
  path: string;
  kind: 'assets' | 'demos';
  size: number;
  warnings?: Issue[];
}

export interface GitEvent {
  time: string;
  dir: string;
  args: string[];
  output?: string;
  error?: string;
  durationNs: number;
}
