/**
 * PoC D — "Workbench". PoC B's shape, reworked from the feedback on it.
 *
 * Three columns in both modes, each column with a fixed role:
 *
 *   left    menu     — editing: the configure panel for the selected parameter
 *                      running: "start from", i.e. prefill from a past build
 *   center  main     — editing: the parameter list, live, click one to configure
 *                      running: the form to fill in
 *   right   context  — editing: the YAML you would commit, copyable and
 *                      downloadable, highlighting the selected parameter
 *                      running: the equivalent `docker run` / `curl`
 *
 * What changed from B:
 *  - The left type palette is gone. It duplicated the type switcher that
 *    already sits in the configure panel, so "add" appends a text parameter
 *    and you change its type there — types live in exactly one place now.
 *  - The configure panel moved from the right to the left. The parameter list
 *    and the panel itself behave as they did in B; only the column changed.
 *  - The YAML took over the right column full-height, with C's line
 *    highlighting, instead of being buried under the panel — plus a download.
 */
import { useMemo, useState } from 'react';
import {
  ArrowLeft,
  Check,
  ChevronDown,
  ChevronUp,
  Copy,
  FileCode2,
  GitCompare,
  GripVertical,
  Pencil,
  Play,
  Plus,
  Rocket,
  Settings2,
  Terminal,
  Trash2,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { PocSwitcher } from './components/PocFrame';
import { RunField } from './components/RunField';
import { ParamEditor } from './components/ParamEditor';
import { EnvPane } from './components/EnvPane';
import { YamlPane } from './components/YamlPane';
import { typeDef } from './params/registry';
import { MOCK_BUILDS, MOCK_JOB, PAST_VALUES, relativeTime } from './mock';
import { useJobDraft } from './useJobDraft';
import type { Build } from './params/types';

type Mode = 'run' | 'edit';
type RunPane = 'docker' | 'curl';

const STATUS_MARK: Record<Build['status'], { glyph: string; tone: string }> = {
  running: { glyph: '▸', tone: 'text-warning' },
  passed: { glyph: '✓', tone: 'text-success' },
  failed: { glyph: '✗', tone: 'text-danger' },
  queued: { glyph: '·', tone: 'text-fg-subtle' },
};

export default function PocWorkbench() {
  const draft = useJobDraft(MOCK_JOB);
  const [mode, setMode] = useState<Mode>('run');
  const [runPane, setRunPane] = useState<RunPane>('docker');
  const [selectedId, setSelectedId] = useState<string | null>(MOCK_JOB.params[0]?.id ?? null);
  const [runFocusId, setRunFocusId] = useState<string | null>(null);
  const [startedFrom, setStartedFrom] = useState<number | null>(null);
  const [queued, setQueued] = useState<number | null>(null);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);

  const { job, values, setValue, errors } = draft;
  const selected = job.params.find((p) => p.id === selectedId) ?? null;

  const dirty = useMemo(
    () => JSON.stringify(job.params) !== JSON.stringify(MOCK_JOB.params),
    [job.params]
  );

  const payload = useMemo(() => {
    const body = Object.fromEntries(
      job.params.filter((p) => p.type !== 'secret').map((p) => [p.name, values[p.name] ?? null])
    );
    return JSON.stringify({ params: body }, null, 2);
  }, [job.params, values]);

  function startFrom(buildId: number | null) {
    if (buildId === null) {
      draft.resetValues();
      setStartedFrom(null);
      return;
    }
    const past = PAST_VALUES[buildId];
    if (!past) return;
    for (const [k, v] of Object.entries(past)) setValue(k, v);
    setStartedFrom(buildId);
  }

  function run() {
    draft.setSubmitted(true);
    if (Object.keys(draft.allErrors).length) return;
    setQueued(413);
    setTimeout(() => setQueued(null), 3000);
  }

  function addInput() {
    // No type choice up front — a new parameter starts as text and is retyped
    // in the configure panel, the one place a type is chosen.
    setSelectedId(draft.addParam('string'));
  }

  function dropOn(index: number) {
    if (dragIndex !== null) draft.reorderParam(dragIndex, index);
    setDragIndex(null);
    setOverIndex(null);
  }

  return (
    <div className='mx-auto max-w-[100rem]'>
      <PocSwitcher />

      {/* header */}
      <div className='mb-4 flex flex-wrap items-center gap-3'>
        <button className='inline-flex items-center gap-1.5 text-sm text-fg-muted hover:text-fg'>
          <ArrowLeft className='h-4 w-4' /> jobs
        </button>
        <span className='text-fg-subtle'>/</span>
        <h1 className='font-mono text-lg font-semibold text-fg'>{job.name}</h1>
        <span className='hidden text-sm text-fg-muted md:inline'>{job.description}</span>

        {dirty && mode === 'edit' && (
          <span className='inline-flex items-center gap-1.5 rounded bg-warning-subtle px-2 py-1 font-mono text-xxs text-warning-fg'>
            <GitCompare className='h-3 w-3' /> drifted from {job.source.commit}
          </span>
        )}

        <div className='ml-auto flex overflow-hidden rounded-md border border-border'>
          {[
            { id: 'run' as const, label: 'Run', icon: Play },
            { id: 'edit' as const, label: 'Edit configuration', icon: Pencil },
          ].map((m) => {
            const Icon = m.icon;
            return (
              <button
                key={m.id}
                onClick={() => setMode(m.id)}
                className={cn(
                  'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs transition-colors',
                  mode === m.id
                    ? 'bg-primary text-fg-onPrimary'
                    : 'text-fg-muted hover:bg-surface-hover hover:text-fg'
                )}
              >
                <Icon className='h-3.5 w-3.5' />
                {m.label}
              </button>
            );
          })}
        </div>
      </div>

      <div
        className={cn(
          'grid gap-4',
          mode === 'edit'
            ? 'xl:grid-cols-[26rem_minmax(0,1fr)_24rem]'
            : 'xl:grid-cols-[15rem_minmax(0,1fr)_25rem]'
        )}
      >
        {/* ============================ LEFT ============================ */}
        <aside className='min-w-0'>
          {mode === 'run' ? (
            <div className='rounded-xl border border-border bg-surface'>
              <div className='border-b border-border px-3 py-2.5'>
                <span className='font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                  start from
                </span>
              </div>
              <div className='p-2'>
                <button
                  onClick={() => startFrom(null)}
                  className={cn(
                    'mb-1 flex w-full items-center gap-2 rounded-md border px-2.5 py-2 text-left text-xs transition-colors',
                    startedFrom === null
                      ? 'border-primary bg-primary-subtle text-primary'
                      : 'border-transparent text-fg-muted hover:border-border hover:text-fg'
                  )}
                >
                  <Rocket className='h-3.5 w-3.5 shrink-0' />
                  Job defaults
                  {startedFrom === null && <Check className='ml-auto h-3.5 w-3.5' />}
                </button>

                <p className='px-2.5 pb-1 pt-2 font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
                  previous builds
                </p>
                {MOCK_BUILDS.map((b) => {
                  const mark = STATUS_MARK[b.status];
                  const active = startedFrom === b.id;
                  const known = Boolean(PAST_VALUES[b.id]);
                  return (
                    <button
                      key={b.id}
                      onClick={() => startFrom(b.id)}
                      disabled={!known}
                      title={known ? `Fill the form with #${b.id}'s values` : 'Values not retained'}
                      className={cn(
                        'flex w-full items-start gap-2 rounded-md border px-2.5 py-1.5 text-left transition-colors',
                        active
                          ? 'border-primary bg-primary-subtle'
                          : 'border-transparent hover:border-border',
                        !known && 'cursor-not-allowed opacity-40'
                      )}
                    >
                      <span className={cn('font-mono text-xs leading-5', mark.tone)}>
                        {mark.glyph}
                      </span>
                      <span className='min-w-0 flex-1'>
                        <span className='block font-mono text-xs text-fg-muted'>#{b.id}</span>
                        <span className='block truncate text-xxs text-fg-subtle'>{b.summary}</span>
                        <span className='block font-mono text-xxs text-fg-subtle'>
                          {relativeTime(b.startedAt)}
                        </span>
                      </span>
                    </button>
                  );
                })}
              </div>
            </div>
          ) : (
            /* The configure panel — B's inspector, moved to the left.
               No inner scroll container on purpose: one would trap the date
               picker and other popovers in a short scroll window. The panel
               grows to its content and the page scrolls instead. */
            <div className='sticky top-4 rounded-xl border border-border bg-surface'>
              <div className='flex items-center gap-2 border-b border-border px-4 py-2.5'>
                <Settings2 className='h-3.5 w-3.5 text-fg-subtle' />
                <span className='font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                  configure
                </span>
                {selected && (
                  <>
                    <span className='text-fg-subtle'>·</span>
                    <span className='truncate font-mono text-xxs uppercase tracking-wider text-primary'>
                      {selected.name}
                    </span>
                  </>
                )}
              </div>
              <div className='p-4'>
                {selected ? (
                  <ParamEditor
                    spec={selected}
                    onChange={(patch) => draft.updateParam(selected.id, patch)}
                    siblings={job.params.filter((p) => p.id !== selected.id).map((p) => p.name)}
                  />
                ) : (
                  <p className='py-10 text-center text-sm text-fg-subtle'>
                    Pick a parameter in the middle to configure it
                  </p>
                )}
              </div>
            </div>
          )}
        </aside>

        {/* =========================== CENTER =========================== */}
        <main className='min-w-0'>
          {mode === 'run' ? (
            <>
              <div className='rounded-xl border border-border bg-surface px-5 py-1'>
                {job.params.map((spec) => (
                  <RunField
                    key={spec.id}
                    spec={spec}
                    value={values[spec.name]}
                    onChange={(v) => setValue(spec.name, v)}
                    error={errors[spec.name]}
                    onFocusCapture={() => setRunFocusId(spec.id)}
                    className='border-b border-dashed border-border py-4 last:border-b-0'
                  />
                ))}
              </div>

              <div className='sticky bottom-0 z-10 mt-3 flex flex-wrap items-center gap-3 rounded-xl border border-border bg-bg-elevated px-4 py-3 shadow-card'>
                <button
                  onClick={run}
                  className='inline-flex items-center gap-2 rounded-md bg-primary px-5 py-2.5 text-sm font-medium text-fg-onPrimary hover:bg-primary-hover'
                >
                  <Play className='h-4 w-4' /> Run build
                </button>
                <span className='font-mono text-xs text-fg-subtle'>
                  queue <span className='text-fg-muted'>{job.queue}</span> · {job.runtime}
                </span>
                {startedFrom !== null && (
                  <span className='rounded bg-surface-hover px-2 py-1 font-mono text-xxs text-fg-muted'>
                    started from #{startedFrom}
                  </span>
                )}
                {queued && (
                  <span className='rounded bg-success-subtle px-2 py-1 font-mono text-xs text-success-fg'>
                    queued #{queued}
                  </span>
                )}
                {draft.submitted && !draft.valid && (
                  <span className='ml-auto font-mono text-xs text-danger'>
                    {Object.keys(draft.allErrors).length} field(s) need attention
                  </span>
                )}
              </div>
            </>
          ) : (
            <div className='rounded-xl border border-border bg-surface'>
              <div className='flex items-center gap-2 border-b border-border px-4 py-2.5'>
                <span className='font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                  parameters · click one to configure
                </span>
                <span className='ml-auto font-mono text-xxs text-fg-subtle'>
                  {job.params.length}
                </span>
              </div>

              <div className='space-y-1 p-3'>
                {job.params.map((spec, i) => {
                  const active = spec.id === selectedId;
                  const Icon = typeDef(spec.type).icon;
                  return (
                    <div
                      key={spec.id}
                      onClick={() => setSelectedId(spec.id)}
                      onDragOver={(e) => {
                        e.preventDefault();
                        setOverIndex(i);
                      }}
                      onDrop={() => dropOn(i)}
                      className={cn(
                        'group relative cursor-pointer rounded-lg border p-3 pl-8 transition-colors',
                        active
                          ? 'border-primary bg-primary-subtle/25'
                          : 'border-transparent hover:border-border hover:bg-surface-hover',
                        dragIndex === i && 'opacity-40',
                        overIndex === i && dragIndex !== null && dragIndex !== i && 'border-primary'
                      )}
                    >
                      {/* drag handle — only the grip is draggable, so text
                          selection inside the controls still works */}
                      <span
                        draggable
                        onDragStart={() => setDragIndex(i)}
                        onDragEnd={() => {
                          setDragIndex(null);
                          setOverIndex(null);
                        }}
                        className='absolute left-1.5 top-3.5 cursor-grab text-fg-subtle opacity-0 transition-opacity group-hover:opacity-100'
                        title='Drag to reorder'
                      >
                        <GripVertical className='h-4 w-4' />
                      </span>

                      <RunField
                        spec={spec}
                        value={values[spec.name]}
                        onChange={(v) => setValue(spec.name, v)}
                      />

                      <div
                        className={cn(
                          'absolute right-2 top-2 flex items-center gap-0.5 rounded border border-border bg-bg-elevated p-0.5 text-fg-subtle transition-opacity',
                          active ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'
                        )}
                      >
                        <span className='px-1 font-mono text-xxs'>
                          <Icon className='h-3.5 w-3.5' />
                        </span>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.moveParam(spec.id, -1);
                          }}
                          disabled={i === 0}
                          className='rounded p-1 hover:bg-surface-hover hover:text-fg disabled:opacity-30'
                          aria-label='Move up'
                        >
                          <ChevronUp className='h-3.5 w-3.5' />
                        </button>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.moveParam(spec.id, 1);
                          }}
                          disabled={i === job.params.length - 1}
                          className='rounded p-1 hover:bg-surface-hover hover:text-fg disabled:opacity-30'
                          aria-label='Move down'
                        >
                          <ChevronDown className='h-3.5 w-3.5' />
                        </button>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.duplicateParam(spec.id);
                          }}
                          className='rounded p-1 hover:bg-surface-hover hover:text-fg'
                          aria-label='Duplicate'
                        >
                          <Copy className='h-3.5 w-3.5' />
                        </button>
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.removeParam(spec.id);
                            if (selectedId === spec.id) setSelectedId(null);
                          }}
                          className='rounded p-1 hover:bg-danger-subtle hover:text-danger'
                          aria-label='Remove'
                        >
                          <Trash2 className='h-3.5 w-3.5' />
                        </button>
                      </div>
                    </div>
                  );
                })}

                {job.params.length === 0 && (
                  <p className='rounded-lg border border-dashed border-border py-10 text-center text-sm text-fg-subtle'>
                    No parameters yet
                  </p>
                )}

                <button
                  onClick={addInput}
                  className='flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-border py-3 text-sm text-fg-muted hover:border-primary hover:text-primary'
                >
                  <Plus className='h-4 w-4' /> Add input
                </button>
              </div>
            </div>
          )}
        </main>

        {/* ============================ RIGHT =========================== */}
        <aside className='flex min-h-[38rem] min-w-0 flex-col'>
          {mode === 'edit' ? (
            <div className='sticky top-4 flex max-h-[calc(100vh-2rem)] flex-col'>
              <div className='mb-2 flex items-center gap-2 rounded-md border border-border px-3 py-1.5'>
                <FileCode2 className='h-3.5 w-3.5 text-primary' />
                <span className='text-xs text-fg'>Definition</span>
                <span className='ml-auto truncate font-mono text-xxs text-fg-subtle'>
                  {job.source.repo}
                </span>
              </div>
              <YamlPane
                job={job}
                focusId={selectedId}
                downloadName={`${job.slug}.yaml`}
                className='min-h-0 flex-1'
              />
              <button
                disabled={!dirty}
                className='mt-2 flex w-full shrink-0 items-center justify-center gap-2 rounded-md bg-primary px-4 py-2.5 text-sm font-medium text-fg-onPrimary hover:bg-primary-hover disabled:cursor-not-allowed disabled:opacity-40'
              >
                <GitCompare className='h-4 w-4' />
                {dirty ? `Open PR against ${job.source.repo}` : 'No changes to commit'}
              </button>
            </div>
          ) : (
            <>
              <div className='mb-2 flex overflow-hidden rounded-md border border-border'>
                {[
                  { id: 'docker' as const, label: 'docker run', icon: Terminal },
                  { id: 'curl' as const, label: 'curl', icon: FileCode2 },
                ].map((p) => {
                  const Icon = p.icon;
                  return (
                    <button
                      key={p.id}
                      onClick={() => setRunPane(p.id)}
                      className={cn(
                        'inline-flex flex-1 items-center justify-center gap-1.5 py-1.5 font-mono text-xs transition-colors',
                        runPane === p.id
                          ? 'bg-primary text-fg-onPrimary'
                          : 'text-fg-muted hover:bg-surface-hover hover:text-fg'
                      )}
                    >
                      <Icon className='h-3.5 w-3.5' />
                      {p.label}
                    </button>
                  );
                })}
              </div>

              <div className='sticky top-4'>
                {runPane === 'docker' ? (
                  <>
                    <EnvPane
                      specs={job.params}
                      values={values}
                      command={job.command}
                      runtime={job.runtime}
                      focusName={job.params.find((p) => p.id === runFocusId)?.name ?? null}
                    />
                    <p className='mt-2 px-1 text-xxs leading-relaxed text-fg-subtle'>
                      Exactly what the worker exports before running{' '}
                      <code className='font-mono text-fg-muted'>{job.command}</code>. Secrets
                      resolve on the worker and never leave the store.
                    </p>
                  </>
                ) : (
                  <div className='rounded-xl border border-border bg-bg-subtle'>
                    <div className='border-b border-border px-3 py-2 font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                      equivalent request
                    </div>
                    <pre className='overflow-auto px-3 py-2.5 font-mono text-xs leading-relaxed text-fg'>
{`curl -X POST \\
  $CORE/api/public/v1/jobs/${job.slug}/builds \\
  -H 'X-API-Key: $LUTE_KEY' \\
  -H 'Idempotency-Key: '"$(uuidgen)" \\
  -d '${payload}'`}
                    </pre>
                    <p className='px-3 pb-3 text-xxs leading-relaxed text-fg-subtle'>
                      The panel and this call share one schema — anything the form rejects,
                      the API rejects.
                    </p>
                  </div>
                )}
              </div>
            </>
          )}
        </aside>
      </div>
    </div>
  );
}
