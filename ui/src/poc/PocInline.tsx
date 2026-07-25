/**
 * PoC A — "Inline".
 *
 * Thesis: triggering a build is the *primary* act of a job page, so it lives
 * there, always visible, never behind a modal. Authoring the schema is the
 * same surface flipped into edit mode: each field turns into an editable card
 * exactly where it sits, so you never lose the sense of the form you are
 * shaping. One page, one mental model, zero navigation.
 *
 * Cost: the run form competes for width with build history, and the edit mode
 * makes a long job page longer.
 */
import { useState } from 'react';
import {
  ChevronDown,
  ChevronUp,
  Copy,
  GitBranch,
  Pencil,
  Play,
  Plus,
  RotateCcw,
  Terminal,
  Trash2,
  X,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { PocSwitcher } from './components/PocFrame';
import { RunField } from './components/RunField';
import { ParamEditor } from './components/ParamEditor';
import { EnvPane } from './components/EnvPane';
import { YamlPane } from './components/YamlPane';
import { typeDef, TYPE_ORDER } from './params/registry';
import { MOCK_BUILDS, MOCK_JOB, formatDuration, relativeTime } from './mock';
import { useJobDraft } from './useJobDraft';
import type { Build } from './params/types';

const BUILD_TONE: Record<Build['status'], string> = {
  running: 'text-warning bg-warning-subtle',
  passed: 'text-success bg-success-subtle',
  failed: 'text-danger bg-danger-subtle',
  queued: 'text-fg-muted bg-surface-hover',
};

export default function PocInline() {
  const draft = useJobDraft(MOCK_JOB);
  const [editing, setEditing] = useState(false);
  const [openId, setOpenId] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);
  const [showEnv, setShowEnv] = useState(false);
  const [queued, setQueued] = useState<number | null>(null);

  const { job, values, setValue, errors } = draft;

  function run() {
    draft.setSubmitted(true);
    if (Object.keys(draft.allErrors).length) return;
    setQueued(413);
    setTimeout(() => setQueued(null), 3000);
  }

  return (
    <div className='mx-auto max-w-6xl'>
      <PocSwitcher />

      {/* job header */}
      <div className='mb-5 flex flex-wrap items-start gap-4'>
        <div className='min-w-0 flex-1'>
          <h1 className='font-mono text-xl font-semibold text-fg'>{job.name}</h1>
          <p className='mt-1 text-sm text-fg-muted'>{job.description}</p>
          <div className='mt-2 flex flex-wrap items-center gap-3 font-mono text-xxs text-fg-subtle'>
            <span className='inline-flex items-center gap-1.5'>
              <GitBranch className='h-3 w-3' /> {job.source.repo}@{job.source.commit}
            </span>
            <span>queue: {job.queue}</span>
            <span>{job.runtime}</span>
          </div>
        </div>
        <button
          onClick={() => {
            setEditing((e) => !e);
            setOpenId(null);
            setAdding(false);
          }}
          className={cn(
            'inline-flex items-center gap-2 rounded-md border px-3 py-2 text-sm transition-colors',
            editing
              ? 'border-primary bg-primary-subtle text-primary'
              : 'border-border text-fg-muted hover:border-border-strong hover:text-fg'
          )}
        >
          <Pencil className='h-4 w-4' />
          {editing ? 'Done editing' : 'Edit inputs'}
        </button>
      </div>

      <div className='grid gap-5 lg:grid-cols-[minmax(0,1fr)_20rem]'>
        {/* --- run form / editor ------------------------------------------- */}
        <div className='rounded-xl border border-border bg-surface'>
          <div className='flex items-center gap-2 border-b border-border px-4 py-3'>
            <h2 className='font-mono text-xs uppercase tracking-wider text-fg-muted'>
              {editing ? 'Inputs — editing schema' : 'New build'}
            </h2>
            {editing && (
              <span className='rounded bg-warning-subtle px-1.5 py-0.5 font-mono text-xxs text-warning-fg'>
                unsaved · commit to {job.source.path}
              </span>
            )}
            {!editing && (
              <button
                onClick={draft.resetValues}
                className='ml-auto inline-flex items-center gap-1.5 font-mono text-xxs text-fg-subtle hover:text-fg'
              >
                <RotateCcw className='h-3 w-3' /> reset to defaults
              </button>
            )}
          </div>

          <div className='divide-y divide-dashed divide-border px-4'>
            {job.params.map((spec, i) => {
              const def = typeDef(spec.type);
              const Icon = def.icon;
              const open = openId === spec.id;

              if (!editing) {
                return (
                  <RunField
                    key={spec.id}
                    spec={spec}
                    value={values[spec.name]}
                    onChange={(v) => setValue(spec.name, v)}
                    error={errors[spec.name]}
                    className='py-4'
                  />
                );
              }

              return (
                <div key={spec.id} className='py-3'>
                  <div className='flex items-center gap-2'>
                    <Icon className='h-4 w-4 shrink-0 text-fg-subtle' />
                    <button
                      onClick={() => setOpenId(open ? null : spec.id)}
                      className='min-w-0 flex-1 text-left'
                    >
                      <span className='text-sm font-semibold text-fg'>{spec.label}</span>
                      <span className='ml-2 font-mono text-xxs text-fg-subtle'>
                        {spec.type} · ${spec.env}
                      </span>
                    </button>
                    <div className='flex items-center gap-0.5 text-fg-subtle'>
                      <button
                        onClick={() => draft.moveParam(spec.id, -1)}
                        disabled={i === 0}
                        className='rounded p-1 hover:bg-surface-hover hover:text-fg disabled:opacity-30'
                        aria-label='Move up'
                      >
                        <ChevronUp className='h-3.5 w-3.5' />
                      </button>
                      <button
                        onClick={() => draft.moveParam(spec.id, 1)}
                        disabled={i === job.params.length - 1}
                        className='rounded p-1 hover:bg-surface-hover hover:text-fg disabled:opacity-30'
                        aria-label='Move down'
                      >
                        <ChevronDown className='h-3.5 w-3.5' />
                      </button>
                      <button
                        onClick={() => draft.duplicateParam(spec.id)}
                        className='rounded p-1 hover:bg-surface-hover hover:text-fg'
                        aria-label='Duplicate'
                      >
                        <Copy className='h-3.5 w-3.5' />
                      </button>
                      <button
                        onClick={() => draft.removeParam(spec.id)}
                        className='rounded p-1 hover:bg-danger-subtle hover:text-danger'
                        aria-label='Remove'
                      >
                        <Trash2 className='h-3.5 w-3.5' />
                      </button>
                    </div>
                  </div>

                  {open ? (
                    <div className='mt-3 rounded-lg border border-primary/30 bg-bg-subtle p-4'>
                      <ParamEditor
                        spec={spec}
                        onChange={(patch) => draft.updateParam(spec.id, patch)}
                        siblings={job.params.filter((p) => p.id !== spec.id).map((p) => p.name)}
                      />
                    </div>
                  ) : (
                    // Collapsed rows still render the real control, greyed out:
                    // the schema list never stops looking like the form.
                    <div className='pointer-events-none mt-2 opacity-50'>
                      <def.Input
                        spec={spec}
                        value={values[spec.name]}
                        onChange={() => {}}
                      />
                    </div>
                  )}
                </div>
              );
            })}
          </div>

          {editing && (
            <div className='border-t border-border px-4 py-3'>
              {adding ? (
                <div className='rounded-lg border border-border bg-bg-subtle p-3'>
                  <div className='mb-2 flex items-center'>
                    <span className='font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
                      add an input
                    </span>
                    <button
                      onClick={() => setAdding(false)}
                      className='ml-auto text-fg-subtle hover:text-fg'
                    >
                      <X className='h-3.5 w-3.5' />
                    </button>
                  </div>
                  <div className='grid grid-cols-2 gap-1.5 sm:grid-cols-3'>
                    {TYPE_ORDER.map((id) => {
                      const t = typeDef(id);
                      const Icon = t.icon;
                      return (
                        <button
                          key={id}
                          onClick={() => {
                            setOpenId(draft.addParam(id));
                            setAdding(false);
                          }}
                          className='flex items-center gap-2 rounded-md border border-border px-2.5 py-2 text-left text-xs text-fg-muted hover:border-primary hover:text-fg'
                        >
                          <Icon className='h-3.5 w-3.5 shrink-0' />
                          {t.label}
                        </button>
                      );
                    })}
                  </div>
                </div>
              ) : (
                <button
                  onClick={() => setAdding(true)}
                  className='flex w-full items-center justify-center gap-2 rounded-md border border-dashed border-border py-2.5 text-sm text-fg-muted hover:border-primary hover:text-primary'
                >
                  <Plus className='h-4 w-4' /> Add input
                </button>
              )}
            </div>
          )}

          {/* action bar */}
          <div className='flex flex-wrap items-center gap-3 rounded-b-xl border-t border-border bg-bg-subtle px-4 py-3'>
            {editing ? (
              <>
                <span className='text-xs text-fg-muted'>
                  Changes produce a YAML diff — commit it to apply.
                </span>
                <button className='ml-auto inline-flex items-center gap-2 rounded-md bg-primary px-3.5 py-2 text-sm font-medium text-fg-onPrimary hover:bg-primary-hover'>
                  Review diff
                </button>
              </>
            ) : (
              <>
                <button
                  onClick={run}
                  className='inline-flex items-center gap-2 rounded-md bg-primary px-4 py-2 text-sm font-medium text-fg-onPrimary hover:bg-primary-hover'
                >
                  <Play className='h-4 w-4' /> Run build
                </button>
                <button
                  onClick={() => setShowEnv((s) => !s)}
                  className='inline-flex items-center gap-2 rounded-md border border-border px-3 py-2 text-sm text-fg-muted hover:border-border-strong hover:text-fg'
                >
                  <Terminal className='h-4 w-4' />
                  {showEnv ? 'Hide' : 'Preview'} command
                </button>
                {queued && (
                  <span className='rounded bg-success-subtle px-2 py-1 font-mono text-xs text-success-fg'>
                    queued #{queued}
                  </span>
                )}
                {draft.submitted && !draft.valid && (
                  <span className='font-mono text-xs text-danger'>
                    {Object.keys(draft.allErrors).length} field(s) need attention
                  </span>
                )}
              </>
            )}
          </div>

          {showEnv && !editing && (
            <div className='px-4 pb-4'>
              <EnvPane
                specs={job.params}
                values={values}
                command={job.command}
                runtime={job.runtime}
              />
            </div>
          )}
        </div>

        {/* --- side column -------------------------------------------------- */}
        <div className='space-y-5'>
          {editing ? (
            <YamlPane job={job} focusId={openId} className='max-h-[36rem]' />
          ) : (
            <div className='rounded-xl border border-border bg-surface'>
              <div className='border-b border-border px-4 py-3'>
                <h2 className='font-mono text-xs uppercase tracking-wider text-fg-muted'>
                  Recent builds
                </h2>
              </div>
              <div className='px-3 py-1'>
                {MOCK_BUILDS.map((b) => (
                  <div
                    key={b.id}
                    className='flex items-center gap-2.5 border-b border-border-subtle py-2.5 last:border-b-0'
                  >
                    <span
                      className={cn(
                        'rounded px-1.5 py-0.5 font-mono text-xxs uppercase',
                        BUILD_TONE[b.status]
                      )}
                    >
                      {b.status === 'running' && <span className='mr-1 animate-pulse'>●</span>}
                      {b.status}
                    </span>
                    <span className='font-mono text-xs text-fg-muted'>#{b.id}</span>
                    <div className='ml-auto min-w-0 text-right'>
                      <div className='truncate text-xs text-fg'>{b.summary}</div>
                      <div className='font-mono text-xxs text-fg-subtle'>
                        {relativeTime(b.startedAt)} · {formatDuration(b.durationMs)}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
