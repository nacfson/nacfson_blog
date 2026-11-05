import React from 'react';
import { PublishPlan, PublishResult } from '../types';

interface PublishDialogProps {
  open: boolean;
  title: string;
  slug: string;
  remote: string;
  publishBranch: string;
  plan: PublishPlan | null;
  loadingPlan: boolean;
  publishing: boolean;
  activeStep: string | null;
  completedSteps: string[];
  result: PublishResult | null;
  error: string | null;
  onClose: () => void;
  onConfirm: () => void;
}

const STEP_ORDER = [
  'save',
  'fetch',
  'reset',
  'squash',
  'check-paths',
  'validate',
  'commit',
  'push',
  'cleanup',
];

function matchStepIndex(stepName: string, steps: string[], unsaved: boolean): number {
  const offset = unsaved ? 0 : -1;
  const idx = STEP_ORDER.indexOf(stepName);
  if (idx === -1) return -1;
  const mapped = idx + offset;
  return mapped >= 0 && mapped < steps.length ? mapped : -1;
}

export const PublishDialog: React.FC<PublishDialogProps> = ({
  open,
  title,
  slug,
  remote,
  publishBranch,
  plan,
  loadingPlan,
  publishing,
  activeStep,
  completedSteps,
  result,
  error,
  onClose,
  onConfirm,
}) => {
  if (!open) return null;

  const issues = plan?.issues ?? [];
  const errors = issues.filter((i) => i.severity === 'error');
  const warnings = issues.filter((i) => i.severity === 'warning');
  const diffLines = (plan?.diff ?? '').split('\n').filter((_, idx, arr) => idx < arr.length - 1 || arr[idx] !== '');
  const adds = diffLines.filter((l) => l.startsWith('+') && !l.startsWith('+++')).length;
  const dels = diffLines.filter((l) => l.startsWith('-') && !l.startsWith('---')).length;

  const steps = plan?.steps ?? [];
  const unsaved = plan?.unsaved ?? false;
  const activeIdx = activeStep ? matchStepIndex(activeStep, steps, unsaved) : -1;
  const doneIndices = new Set(
    completedSteps.map((s) => matchStepIndex(s, steps, unsaved)).filter((n) => n >= 0),
  );

  const canConfirm = Boolean(plan && !plan.blocking && !publishing && !result && !loadingPlan);

  return (
    <div
      className="modal"
      onClick={(e) => {
        if (e.target === e.currentTarget && !publishing) {
          onClose();
        }
      }}
    >
      <div className="dialog" role="dialog" aria-modal="true" aria-labelledby="dlgTitle">
        <div className="dlg-h">
          <h2 id="dlgTitle">Publish to blog.nacfson.cloud</h2>
          <p id="dlgSub">
            {title || 'Untitled'} · {plan?.path || `content/posts/${slug}.md`}
          </p>
        </div>

        <div className="dlg-b">
          {loadingPlan && <p className="text-xs text-stone-500">Computing publish plan…</p>}
          {error && (
            <div role="alert" className="rounded border border-stone-900 bg-stone-100 p-3 text-xs font-mono text-stone-950">
              ✗ {error}
            </div>
          )}

          {plan && (
            <>
              <div>
                <p className="sect">Checks</p>
                <ul className="checks">
                  <li>
                    <span>{errors.length === 0 ? '✓' : '●'}</span>
                    <span>
                      {errors.length > 0 ? (
                        <>
                          <b>{errors.length} validation error(s)</b>:{' '}
                          {errors.map((e) => e.message).join('; ')}
                        </>
                      ) : (
                        <>
                          Post passes the production parser (<code>internal/content</code>)
                        </>
                      )}
                    </span>
                  </li>
                  <li>
                    <span>{errors.length === 0 ? '✓' : '●'}</span>
                    <span>
                      All posts on <code>{publishBranch}</code> + this one still index cleanly (no
                      duplicate slugs)
                    </span>
                  </li>
                  <li>
                    <span>✓</span>
                    <span>
                      Commit touches <code>content/**</code> only: no image rebuild will be
                      triggered
                    </span>
                  </li>
                  {plan.unsaved && (
                    <li>
                      <span>✓</span>
                      <span>Unsaved changes will be saved (committed) first</span>
                    </li>
                  )}
                  {warnings.length > 0 && (
                    <li>
                      <span>✓</span>
                      <span>{warnings.length} warning(s), not blocking</span>
                    </li>
                  )}
                </ul>
              </div>

              <div>
                <p className="sect">
                  Changes · +{adds} −{dels}
                </p>
                <div className="diff" data-testid="publish-diff">
                  <div className="hdr">
                    {plan.files.length ? plan.files.join(', ') : plan.path}
                  </div>
                  {diffLines.length === 0 ? (
                    <div> (no diff)</div>
                  ) : (
                    diffLines.map((line, idx) => {
                      const cls =
                        line.startsWith('+') && !line.startsWith('+++')
                          ? 'add'
                          : line.startsWith('-') && !line.startsWith('---')
                            ? 'del'
                            : line.startsWith('@@') || line.startsWith('diff ')
                              ? 'hdr'
                              : '';
                      return (
                        <div key={idx} className={cls}>
                          {line}
                        </div>
                      );
                    })
                  )}
                </div>
              </div>

              <div>
                <p className="sect">Will run</p>
                <ol className="steps">
                  {steps.map((stepCmd, idx) => {
                    const isDone = Boolean(result) || doneIndices.has(idx) || (activeIdx > idx);
                    const isRun = !isDone && publishing && activeIdx === idx;
                    return (
                      <li key={idx} className={isDone ? 'done' : isRun ? 'run' : ''}>
                        {stepCmd}
                      </li>
                    );
                  })}
                </ol>
              </div>
            </>
          )}
        </div>

        <div className="dlg-f">
          <span className="warn">
            {result ? (
              <>
                <b>Published {result.commit.slice(0, 7)}</b> to {remote}/{publishBranch}. Live in
                ~30s at /post/?slug={result.slug}
              </>
            ) : (
              <>
                Pushes directly to{' '}
                <b>
                  {remote}/{publishBranch}
                </b>
                . Flux syncs the live site in ~30s.
              </>
            )}
          </span>
          <button type="button" className="btn" disabled={publishing} onClick={onClose}>
            {result ? 'Done' : 'Cancel'}
          </button>
          <button
            type="button"
            className="btn primary"
            disabled={!canConfirm}
            onClick={onConfirm}
          >
            {result ? 'Pushed' : `Push to ${publishBranch}`}
          </button>
        </div>
      </div>
    </div>
  );
};
