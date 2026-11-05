import React, { useCallback, useEffect, useRef, useState } from 'react';
import { api } from './api';
import {
  Document,
  DraftState,
  DraftStatus,
  DraftView,
  Frontmatter,
  GitEvent,
  Issue,
  PublishedPost,
  PublishPlan,
  PublishResult,
} from './types';
import {
  dotClassForState,
  formatHeaderStatus,
  postFilePath,
  slugify,
} from './utils';
import { Sidebar } from './components/Sidebar';
import { FrontmatterForm } from './components/FrontmatterForm';
import { Editor, EditorHandle } from './components/Editor';
import { Preview } from './components/Preview';
import { ChecksPanel } from './components/ChecksPanel';
import { GitLog } from './components/GitLog';
import { PublishDialog } from './components/PublishDialog';

const EMPTY_FRONTMATTER: Frontmatter = {
  title: '',
  slug: '',
  date: new Date().toISOString().slice(0, 10),
  topic: '',
  project: '',
  summary: '',
  tags: [],
};

function nowHHMMSS(): string {
  return new Date().toTimeString().slice(0, 8);
}

export const App: React.FC = () => {
  const [publishBranch, setPublishBranch] = useState('main');
  const [remote, setRemote] = useState('origin');
  const [drafts, setDrafts] = useState<DraftView[]>([]);
  const [published, setPublished] = useState<PublishedPost[]>([]);

  const [currentDraft, setCurrentDraft] = useState<DraftStatus | null>(null);
  const [doc, setDoc] = useState<Document>({
    frontmatter: EMPTY_FRONTMATTER,
    body: '',
  });
  const [previewBody, setPreviewBody] = useState('');
  const [issues, setIssues] = useState<Issue[]>([]);
  const [slugTouched, setSlugTouched] = useState(true);
  const [autosavedAt, setAutosavedAt] = useState<string | null>(null);
  const [localDirty, setLocalDirty] = useState(false);

  const [events, setEvents] = useState<GitEvent[]>([]);
  const [toastMsg, setToastMsg] = useState<string | null>(null);

  // Publish modal state
  const [publishOpen, setPublishOpen] = useState(false);
  const [publishPlan, setPublishPlan] = useState<PublishPlan | null>(null);
  const [loadingPlan, setLoadingPlan] = useState(false);
  const [publishing, setPublishing] = useState(false);
  const [activeStep, setActiveStep] = useState<string | null>(null);
  const [completedSteps, setCompletedSteps] = useState<string[]>([]);
  const [publishResult, setPublishResult] = useState<PublishResult | null>(null);
  const [publishError, setPublishError] = useState<string | null>(null);

  const editorRef = useRef<EditorHandle>(null);
  const titleInputRef = useRef<HTMLInputElement>(null);
  const toastTimerRef = useRef<number | null>(null);
  const autosaveTimerRef = useRef<number | null>(null);
  const previewTimerRef = useRef<number | null>(null);
  const latestDocRef = useRef<{ key: string | null; doc: Document; dirty: boolean }>({
    key: null,
    doc,
    dirty: false,
  });
  latestDocRef.current = {
    key: currentDraft?.key ?? null,
    doc,
    dirty: localDirty,
  };

  const showToast = useCallback((msg: string) => {
    setToastMsg(msg);
    if (toastTimerRef.current) {
      window.clearTimeout(toastTimerRef.current);
    }
    toastTimerRef.current = window.setTimeout(() => {
      setToastMsg(null);
    }, 2200);
  }, []);

  const refreshState = useCallback(async () => {
    const st = await api.getState();
    setPublishBranch(st.publishBranch || 'main');
    setRemote(st.remote || 'origin');
    setDrafts(st.drafts || []);
    setPublished(st.published || []);
    return st;
  }, []);

  const applyDraftResponse = useCallback(
    (res: { draft: DraftStatus; document: Document; issues: Issue[] }, isBrandNew = false) => {
      setCurrentDraft(res.draft);
      setDoc(res.document);
      setPreviewBody(res.document.body);
      setIssues(res.issues || []);
      setSlugTouched(!isBrandNew && Boolean(res.document.frontmatter.title));
      setLocalDirty(false);
    },
    [],
  );

  const flushAutosave = useCallback(async () => {
    if (autosaveTimerRef.current) {
      window.clearTimeout(autosaveTimerRef.current);
      autosaveTimerRef.current = null;
    }
    const { key, doc: pendingDoc, dirty } = latestDocRef.current;
    if (!key || !dirty) return;
    try {
      const res = await api.putDraft(key, pendingDoc);
      setCurrentDraft(res.draft);
      setIssues(res.issues || []);
      setAutosavedAt(nowHHMMSS());
      setLocalDirty(false);
      await refreshState();
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Autosave failed');
    }
  }, [refreshState, showToast]);

  // Initial load
  useEffect(() => {
    let cancelled = false;
    (async () => {
      try {
        const st = await refreshState();
        if (cancelled) return;
        const firstKey =
          st.active || st.drafts.find((d) => d.state !== 'live')?.key || st.drafts[0]?.key;
        if (firstKey) {
          const dr = await api.getDraft(firstKey);
          if (!cancelled) applyDraftResponse(dr, dr.draft.state === 'new');
        }
      } catch (err) {
        if (!cancelled) {
          showToast(err instanceof Error ? err.message : 'Failed to load studio state');
        }
      }
    })();
    return () => {
      cancelled = true;
    };
  }, [applyDraftResponse, refreshState, showToast]);

  // SSE subscription for git log
  useEffect(() => {
    if (typeof EventSource === 'undefined') return;
    const es = new EventSource('/studio/api/events');
    es.onmessage = (e) => {
      try {
        const ev = JSON.parse(e.data) as GitEvent;
        setEvents((prev) => [...prev.slice(-99), ev]);
      } catch {
        // ignore malformed event
      }
    };
    return () => {
      es.close();
    };
  }, []);

  // Debounce preview updates at 150ms
  useEffect(() => {
    if (previewTimerRef.current) {
      window.clearTimeout(previewTimerRef.current);
    }
    previewTimerRef.current = window.setTimeout(() => {
      setPreviewBody(doc.body);
    }, 150);
    return () => {
      if (previewTimerRef.current) {
        window.clearTimeout(previewTimerRef.current);
      }
    };
  }, [doc.body]);

  const scheduleAutosave = useCallback(
    (key: string, nextDoc: Document) => {
      setLocalDirty(true);
      if (autosaveTimerRef.current) {
        window.clearTimeout(autosaveTimerRef.current);
      }
      autosaveTimerRef.current = window.setTimeout(async () => {
        autosaveTimerRef.current = null;
        try {
          const res = await api.putDraft(key, nextDoc);
          setCurrentDraft(res.draft);
          setIssues(res.issues || []);
          setAutosavedAt(nowHHMMSS());
          setLocalDirty(false);
          await refreshState();
        } catch (err) {
          showToast(err instanceof Error ? err.message : 'Autosave failed');
        }
      }, 700);
    },
    [refreshState, showToast],
  );

  const handleFrontmatterChange = (nextFm: Frontmatter, slugManuallyEdited = false) => {
    let nextSlugTouched = slugTouched;
    const adjustedFm = { ...nextFm };
    if (slugManuallyEdited) {
      nextSlugTouched = true;
    } else if (!slugTouched) {
      adjustedFm.slug = slugify(nextFm.title) || currentDraft?.key || 'untitled';
    }
    setSlugTouched(nextSlugTouched);
    const nextDoc: Document = { frontmatter: adjustedFm, body: doc.body };
    setDoc(nextDoc);
    if (currentDraft) {
      scheduleAutosave(currentDraft.key, nextDoc);
    }
  };

  const handleBodyChange = (nextBody: string) => {
    const nextDoc: Document = { frontmatter: doc.frontmatter, body: nextBody };
    setDoc(nextDoc);
    if (currentDraft) {
      scheduleAutosave(currentDraft.key, nextDoc);
    }
  };

  const handleNewDraft = async () => {
    await flushAutosave();
    try {
      const res = await api.createDraft('');
      applyDraftResponse(res, true);
      setAutosavedAt(null);
      await refreshState();
      setTimeout(() => titleInputRef.current?.focus(), 0);
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Could not create draft');
    }
  };

  const handleSelectDraft = async (key: string) => {
    if (currentDraft?.key === key) return;
    await flushAutosave();
    try {
      await api.setActive(key);
      const res = await api.getDraft(key);
      applyDraftResponse(res, res.draft.state === 'new' && !res.document.frontmatter.title);
      setAutosavedAt(null);
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Could not load draft');
    }
  };

  const handleSelectPublished = async (slug: string) => {
    await flushAutosave();
    // If a draft already exists for this slug, open it directly.
    const existing = drafts.find((d) => d.key === slug || d.slug === slug);
    if (existing) {
      await handleSelectDraft(existing.key);
      return;
    }
    try {
      const res = await api.editPublished(slug);
      applyDraftResponse(res, false);
      setAutosavedAt(null);
      await refreshState();
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Could not open published post');
    }
  };

  const handleSave = useCallback(async () => {
    if (!currentDraft) return;
    await flushAutosave();
    try {
      const sres = await api.saveDraft(currentDraft.key);
      const dr = await api.getDraft(currentDraft.key);
      applyDraftResponse(dr, false);
      setAutosavedAt(null);
      await refreshState();
      if (sres.nothingToCommit) {
        showToast('Nothing to commit');
      } else {
        showToast(`Saved: committed ${sres.commit ?? ''} on ${dr.draft.branch}`);
      }
    } catch (err) {
      // Refresh draft to ensure latest lint issues are displayed
      try {
        const vres = await api.validateDraft(currentDraft.key, latestDocRef.current.doc);
        setIssues(vres.issues || []);
      } catch {
        // ignore
      }
      showToast(err instanceof Error ? err.message : 'Save failed');
    }
  }, [applyDraftResponse, currentDraft, flushAutosave, refreshState, showToast]);

  const handleDiscard = async () => {
    if (!currentDraft) return;
    const confirmed = window.confirm(
      `Discard draft "${doc.frontmatter.title || currentDraft.key}" (${currentDraft.branch})? Unpublished commits and worktree changes will be permanently deleted.`,
    );
    if (!confirmed) return;
    if (autosaveTimerRef.current) {
      window.clearTimeout(autosaveTimerRef.current);
      autosaveTimerRef.current = null;
    }
    try {
      await api.discardDraft(currentDraft.key);
      setCurrentDraft(null);
      const st = await refreshState();
      const nextKey = st.drafts.find((d) => d.state !== 'live')?.key || st.drafts[0]?.key;
      if (nextKey) {
        const dr = await api.getDraft(nextKey);
        applyDraftResponse(dr, false);
      } else {
        setDoc({ frontmatter: EMPTY_FRONTMATTER, body: '' });
        setPreviewBody('');
        setIssues([]);
      }
      showToast('Draft discarded');
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Discard failed');
    }
  };

  const handleOpenPublish = async () => {
    if (!currentDraft) return;
    await flushAutosave();
    setPublishOpen(true);
    setPublishPlan(null);
    setPublishResult(null);
    setPublishError(null);
    setActiveStep(null);
    setCompletedSteps([]);
    setLoadingPlan(true);
    try {
      const plan = await api.getPublishPlan(currentDraft.key);
      setPublishPlan(plan);
      setIssues(plan.issues || []);
    } catch (err) {
      setPublishError(err instanceof Error ? err.message : 'Failed to load publish plan');
    } finally {
      setLoadingPlan(false);
    }
  };

  const handleConfirmPublish = async () => {
    if (!currentDraft || publishing) return;
    setPublishing(true);
    setPublishError(null);
    setActiveStep(null);
    setCompletedSteps([]);
    try {
      const res = await api.publishDraft(currentDraft.key, (step) => {
        setActiveStep((prev) => {
          if (prev && prev !== step) {
            setCompletedSteps((done) => (done.includes(prev) ? done : [...done, prev]));
          }
          return step;
        });
      });
      setActiveStep(null);
      setPublishResult(res);
      const st = await refreshState();
      const remaining = st.drafts.find((d) => d.state !== 'live');
      if (remaining) {
        const dr = await api.getDraft(remaining.key);
        applyDraftResponse(dr, false);
      } else {
        setCurrentDraft((prev) => (prev ? { ...prev, state: 'live', ahead: 0 } : null));
      }
      showToast(`Published ${res.commit.slice(0, 7)} to ${remote}/${res.publishBranch}`);
    } catch (err) {
      setPublishError(err instanceof Error ? err.message : 'Publish failed');
    } finally {
      setPublishing(false);
    }
  };

  const handleUploadFile = async (file: File, insertBlock: boolean) => {
    if (!currentDraft) return;
    await flushAutosave();
    try {
      const res = await api.uploadAsset(currentDraft.key, file);
      if (insertBlock) {
        if (res.kind === 'assets') {
          const snippet = `\n\`\`\`blogPicture\n{\n  "src": "${res.url}",\n  "alt": "Describe ${file.name}",\n  "caption": "Figure: …",\n  "aspectRatio": "1200/520"\n}\n\`\`\`\n`;
          editorRef.current?.insertSnippet(snippet);
        } else {
          const snippet = `\n\`\`\`blogIframeTabs\n{\n  "mountStrategy": "click-or-center",\n  "height": "520px",\n  "tabs": [\n    {\n      "label": "Live demo",\n      "captionTitle": "${file.name}",\n      "src": "${res.url}",\n      "poster": "/content/assets/your-demo-poster.svg"\n    }\n  ]\n}\n\`\`\`\n`;
          editorRef.current?.insertSnippet(snippet);
        }
      }
      if (res.warnings?.length) {
        setIssues((prev) => [...prev, ...res.warnings!]);
      }
      await refreshState();
      showToast(`Uploaded ${res.url}`);
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Upload failed');
    }
  };

  const handleSelectIssue = (issue: Issue) => {
    if (issue.line) {
      editorRef.current?.jumpToLine(issue.line);
      return;
    }
    if (issue.field) {
      const el = document.getElementById(`f-${issue.field}`);
      el?.focus();
    }
  };

  // Global keyboard shortcuts: Ctrl/Cmd+S and Escape
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 's') {
        e.preventDefault();
        void handleSave();
      } else if (e.key === 'Escape' && publishOpen && !publishing) {
        setPublishOpen(false);
      }
    };
    window.addEventListener('keydown', onKeyDown);
    return () => window.removeEventListener('keydown', onKeyDown);
  }, [handleSave, publishOpen, publishing]);

  const effectiveState: DraftState = currentDraft
    ? localDirty
      ? 'unsaved'
      : currentDraft.state
    : 'new';
  const canSave = Boolean(
    currentDraft && (localDirty || currentDraft.state === 'unsaved' || currentDraft.state === 'new'),
  );
  const canPublish = Boolean(currentDraft && effectiveState !== 'live');
  const displayPath = currentDraft
    ? postFilePath(doc.frontmatter.date, doc.frontmatter.slug || currentDraft.key)
    : 'no draft selected';

  return (
    <div className="studio-shell">
      <header className="top">
        <div className="brand">
          nacfson<span className="sep">/</span>Blog Studio
          <span className="local">127.0.0.1:4321 · local only</span>
        </div>

        <div className="branch" title="Draft branch (own worktree)">
          <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2">
            <circle cx="6" cy="5" r="2" />
            <circle cx="6" cy="19" r="2" />
            <circle cx="18" cy="8" r="2" />
            <path d="M6 7v10M18 10c0 4-6 3-12 7" />
          </svg>
          <span data-testid="header-branch">
            {currentDraft ? currentDraft.branch : `studio/…`}
          </span>
        </div>

        {currentDraft && (
          <div className="status" data-testid="header-status">
            <span className={dotClassForState(effectiveState)} />
            <span>
              {formatHeaderStatus(
                effectiveState,
                currentDraft.ahead,
                publishBranch,
                autosavedAt,
              )}
            </span>
          </div>
        )}

        <div className="spacer" />

        {currentDraft && (
          <button
            type="button"
            className="btn"
            onClick={handleDiscard}
            title="Discard draft branch and worktree"
          >
            Discard
          </button>
        )}

        <button
          type="button"
          className="btn"
          disabled={!canSave}
          onClick={() => void handleSave()}
        >
          Save <kbd>Ctrl S</kbd>
        </button>

        <button
          type="button"
          className="btn primary"
          disabled={!canPublish}
          onClick={() => void handleOpenPublish()}
        >
          Publish…
        </button>
      </header>

      <div className="app">
        <Sidebar
          publishBranch={publishBranch}
          remote={remote}
          drafts={drafts}
          published={published}
          activeKey={currentDraft?.key ?? null}
          activePublishedSlug={currentDraft?.state === 'live' ? doc.frontmatter.slug : null}
          onNewDraft={() => void handleNewDraft()}
          onSelectDraft={(k) => void handleSelectDraft(k)}
          onSelectPublished={(s) => void handleSelectPublished(s)}
        />

        <main className="main">
          <FrontmatterForm
            frontmatter={doc.frontmatter}
            slugTouched={slugTouched}
            issues={issues}
            disabled={!currentDraft}
            titleInputRef={titleInputRef}
            onChange={handleFrontmatterChange}
          />

          <div className="split">
            <Editor
              ref={editorRef}
              value={doc.body}
              filePath={displayPath}
              disabled={!currentDraft}
              onChange={handleBodyChange}
              onUploadFile={(file, ins) => void handleUploadFile(file, ins)}
            />
            <Preview frontmatter={doc.frontmatter} markdown={previewBody} />
          </div>

          <section className="bottom">
            <ChecksPanel issues={issues} onSelectIssue={handleSelectIssue} />
            <GitLog events={events} />
          </section>
        </main>
      </div>

      <PublishDialog
        open={publishOpen}
        title={doc.frontmatter.title}
        slug={doc.frontmatter.slug || currentDraft?.key || ''}
        remote={remote}
        publishBranch={publishBranch}
        plan={publishPlan}
        loadingPlan={loadingPlan}
        publishing={publishing}
        activeStep={activeStep}
        completedSteps={completedSteps}
        result={publishResult}
        error={publishError}
        onClose={() => setPublishOpen(false)}
        onConfirm={() => void handleConfirmPublish()}
      />

      <div className={`toast${toastMsg ? ' on' : ''}`} role="status" aria-live="polite">
        {toastMsg}
      </div>
    </div>
  );
};
