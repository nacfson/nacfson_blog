import {
  AssetResult,
  Document,
  DraftResponse,
  Issue,
  PublishPlan,
  PublishResult,
  SaveResponse,
  StateResponse,
} from './types';

async function request<T>(method: string, path: string, body?: unknown): Promise<T> {
  const headers: Record<string, string> = {};
  let payload: BodyInit | undefined;
  if (body !== undefined) {
    headers['Content-Type'] = 'application/json';
    payload = JSON.stringify(body);
  } else if (method !== 'GET' && method !== 'HEAD') {
    headers['Content-Type'] = 'application/json';
  }
  const res = await fetch(path, {
    method,
    headers,
    body: payload,
     sameOrigin: true,
  } as RequestInit);
  if (res.status === 204) {
    return undefined as T;
  }
  const data = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
  if (!res.ok) {
    throw new Error(data.error || `HTTP ${res.status}`);
  }
  return data as T;
}

export const api = {
  getState: () => request<StateResponse>('GET', '/studio/api/state'),
  createDraft: (title = '') =>
    request<DraftResponse>('POST', '/studio/api/drafts', { title }),
  editPublished: (slug: string) =>
    request<DraftResponse>('POST', `/studio/api/drafts/from-post/${encodeURIComponent(slug)}`),
  getDraft: (key: string) =>
    request<DraftResponse>('GET', `/studio/api/drafts/${encodeURIComponent(key)}`),
  putDraft: (key: string, doc: Document) =>
    request<DraftResponse>('PUT', `/studio/api/drafts/${encodeURIComponent(key)}`, doc),
  validateDraft: (key: string, doc: Document) =>
    request<{ issues: Issue[] }>(
      'POST',
      `/studio/api/drafts/${encodeURIComponent(key)}/validate`,
      doc,
    ),
  saveDraft: (key: string) =>
    request<SaveResponse>('POST', `/studio/api/drafts/${encodeURIComponent(key)}/save`),
  discardDraft: (key: string) =>
    request<void>('DELETE', `/studio/api/drafts/${encodeURIComponent(key)}`, { confirm: key }),
  getPublishPlan: (key: string) =>
    request<PublishPlan>('GET', `/studio/api/drafts/${encodeURIComponent(key)}/publish-plan`),
  setActive: (key: string) =>
    request<void>('PUT', '/studio/api/active', { key }),

  async uploadAsset(key: string, file: File): Promise<AssetResult> {
    const form = new FormData();
    form.append('file', file);
    const res = await fetch(`/studio/api/drafts/${encodeURIComponent(key)}/assets`, {
      method: 'POST',
      body: form,
    });
    const data = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    if (!res.ok) {
      throw new Error(data.error || `HTTP ${res.status}`);
    }
    return data as AssetResult;
  },

  async publishDraft(
    key: string,
    onStep: (step: string) => void,
  ): Promise<PublishResult> {
    const res = await fetch(`/studio/api/drafts/${encodeURIComponent(key)}/publish`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ confirm: key }),
    });
    if (!res.ok && !res.body) {
      const data = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
      throw new Error(data.error || `HTTP ${res.status}`);
    }
    const reader = res.body?.getReader();
    if (!reader) {
      throw new Error('Publish stream unavailable');
    }
    const decoder = new TextDecoder();
    let buffer = '';
    let finalResult: PublishResult | null = null;

    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';
      for (const line of lines) {
        if (!line.trim()) continue;
        const msg = JSON.parse(line) as {
          step?: string;
          result?: PublishResult;
          error?: string;
        };
        if (msg.step) {
          onStep(msg.step);
        }
        if (msg.error) {
          throw new Error(msg.error);
        }
        if (msg.result) {
          finalResult = msg.result;
        }
      }
    }
    if (buffer.trim()) {
      const msg = JSON.parse(buffer) as {
        step?: string;
        result?: PublishResult;
        error?: string;
      };
      if (msg.step) onStep(msg.step);
      if (msg.error) throw new Error(msg.error);
      if (msg.result) finalResult = msg.result;
    }
    if (!finalResult) {
      throw new Error('Publish stream ended without a result');
    }
    return finalResult;
  },
};
