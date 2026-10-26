import { Topic } from './types';

/** The four interest topics a post can belong to (mirrors internal/content/store.go). */
export const TOPICS: { id: Topic; label: string; short: string }[] = [
  { id: 'cloud-native', label: 'Cloud-Native & Kubernetes', short: 'Cloud-Native' },
  { id: 'zero-trust', label: 'Zero-Trust Security', short: 'Zero-Trust' },
  { id: 'ai-agents', label: 'AI & LLM Agents', short: 'AI Agents' },
  { id: 'open-source', label: 'Open Source & Community', short: 'Open Source' },
];

export function topicLabel(id: string, variant: 'label' | 'short' = 'label'): string {
  const t = TOPICS.find((x) => x.id === id);
  return t ? t[variant] : id;
}

export function isTopic(id: string | null | undefined): id is Topic {
  return !!id && TOPICS.some((t) => t.id === id);
}
