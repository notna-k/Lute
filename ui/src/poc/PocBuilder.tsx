/**
 * PoC B — "Builder".
 *
 * Thesis: running and designing are different jobs, so give each a whole page.
 *
 *  - Run mode is a single focused column with a sticky action bar, plus the
 *    thing Jenkins never gives you: **rerun-with**, prefilling the form from a
 *    previous build's exact values.
 *  - Design mode is the classic three-pane form builder — palette, canvas,
 *    inspector — which scales to a schema with twenty parameters where an
 *    inline accordion would not.
 *
 * Cost: a route away from the job page, and two modes to keep coherent.
 */
import { useState } from 'react';
import {
  ArrowLeft,
  ChevronDown,
  ChevronUp,
  Copy,
  History,
  LayoutList,
  Play,
  Plus,
  Terminal,
  Trash2,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { PocSwitcher } from './components/PocFrame';
import { RunField } from './components/RunField';
import { ParamEditor } from './components/ParamEditor';
import { EnvPane } from './components/EnvPane';
import { YamlPane } from './components/YamlPane';
import { typeDef, TYPE_ORDER } from './params/registry';
import { MOCK_BUILDS, MOCK_JOB, PAST_VALUES, relativeTime } from './mock';
import { useJobDraft } from './useJobDraft';

export default function PocBuilder() {
  const draft = useJobDraft(MOCK_JOB);
  const [mode, setMode] = useState<'run' | 'design'>('run');
  const [selectedId, setSelectedId] = useState<string | null>(MOCK_JOB.params[0]?.id ?? null);
  const [prefilled, setPrefilled] = useState<number | null>(null);
  const [queued, setQueued] = useState<number | null>(null);

  const { job, values, setValue, errors } = draft;
  const selected = job.params.find((p) => p.id === selectedId) ?? null;

  function prefillFrom(buildId: number) {
    const past = PAST_VALUES[buildId];
    if (!past) return;
    for (const [k, v] of Object.entries(past)) setValue(k, v);
    setPrefilled(buildId);
  }

  function run() {
    draft.setSubmitted(true);
    if (Object.keys(draft.allErrors).length) return;
    setQueued(413);
    setTimeout(() => setQueued(null), 3000);
  }

  return (
    <div className='mx-auto max-w-7xl'>
      <PocSwitcher />

      {/* page header */}
      <div className='mb-5 flex flex-wrap items-center gap-3'>
        <button className='inline-flex items-center gap-1.5 text-sm text-fg-muted hover:text-fg'>
          <ArrowLeft className='h-4 w-4' /> {job.name}
        </button>
        <span className='text-fg-subtle'>/</span>
        <h1 className='text-lg font-semibold text-fg'>
          {mode === 'run' ? 'New build' : 'Input schema'}
        </h1>
        <div className='ml-auto flex overflow-hidden rounded-md border border-border'>
          {(['run', 'design'] as const).map((m) => (
            <button
              key={m}
              onClick={() => setMode(m)}
              className={cn(
                'inline-flex items-center gap-1.5 px-3 py-1.5 text-xs transition-colors',
                mode === m
                  ? 'bg-primary text-fg-onPrimary'
                  : 'text-fg-muted hover:bg-surface-hover hover:text-fg'
              )}
            >
              {m === 'run' ? <Play className='h-3.5 w-3.5' /> : <LayoutList className='h-3.5 w-3.5' />}
              {m === 'run' ? 'Run' : 'Design form'}
            </button>
          ))}
        </div>
      </div>

      {mode === 'run' ? (
        <div className='grid gap-6 lg:grid-cols-[minmax(0,1fr)_18rem]'>
          <div>
            {/* rerun-with strip */}
            <div className='mb-4 flex flex-wrap items-center gap-2 rounded-lg border border-border bg-bg-subtle px-3 py-2'>
              <History className='h-3.5 w-3.5 text-fg-subtle' />
              <span className='font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
                start from
              </span>
              <button
                onClick={() => {
                  draft.resetValues();
                  setPrefilled(null);
                }}
                className={cn(
                  'rounded border px-2 py-0.5 font-mono text-xxs transition-colors',
                  prefilled === null
                    ? 'border-primary bg-primary-subtle text-primary'
                    : 'border-border text-fg-muted hover:text-fg'
                )}
              >
                defaults
              </button>
              {Object.keys(PAST_VALUES).map((id) => (
                <button
                  key={id}
                  onClick={() => prefillFrom(Number(id))}
                  className={cn(
                    'rounded border px-2 py-0.5 font-mono text-xxs transition-colors',
                    prefilled === Number(id)
                      ? 'border-primary bg-primary-subtle text-primary'
                      : 'border-border text-fg-muted hover:text-fg'
                  )}
                >
                  #{id}
                </button>
              ))}
              <span className='ml-auto text-xxs text-fg-subtle'>
                Prefills every input from that build.
              </span>
            </div>

            <div className='rounded-xl border border-border bg-surface px-6 py-2'>
              {job.params.map((spec) => (
                <RunField
                  key={spec.id}
                  spec={spec}
                  value={values[spec.name]}
                  onChange={(v) => setValue(spec.name, v)}
                  error={errors[spec.name]}
                  className='border-b border-dashed border-border py-5 last:border-b-0'
                />
              ))}
            </div>

            {/* sticky action bar */}
            <div className='sticky bottom-0 z-10 mt-4 flex flex-wrap items-center gap-3 rounded-xl border border-border bg-bg-elevated px-4 py-3 shadow-card'>
              <button
                onClick={run}
                className='inline-flex items-center gap-2 rounded-md bg-primary px-5 py-2.5 text-sm font-medium text-fg-onPrimary hover:bg-primary-hover'
              >
                <Play className='h-4 w-4' /> Run build
              </button>
              <span className='font-mono text-xs text-fg-subtle'>
                queue <span className='text-fg-muted'>{job.queue}</span> · {job.runtime}
              </span>
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
          </div>

          <div className='space-y-4'>
            <EnvPane
              specs={job.params}
              values={values}
              command={job.command}
              runtime={job.runtime}
            />
            <div className='rounded-xl border border-border bg-surface p-4'>
              <h3 className='mb-2 font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                Trigger via API
              </h3>
              <pre className='overflow-x-auto rounded bg-bg-subtle p-2.5 font-mono text-xxs leading-relaxed text-fg-muted'>
{`POST /api/public/v1/jobs/
     ${job.slug}/builds
{
  "params": { … }
}`}
              </pre>
              <p className='mt-2 text-xxs text-fg-subtle'>
                Same schema validates this payload server-side.
              </p>
            </div>
            <div className='rounded-xl border border-border bg-surface p-4'>
              <h3 className='mb-2 font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                Last builds
              </h3>
              {MOCK_BUILDS.slice(0, 4).map((b) => (
                <div key={b.id} className='flex items-center gap-2 py-1 font-mono text-xxs'>
                  <span className='text-fg-muted'>#{b.id}</span>
                  <span className='truncate text-fg-subtle'>{b.summary}</span>
                  <span className='ml-auto shrink-0 text-fg-subtle'>
                    {relativeTime(b.startedAt)}
                  </span>
                </div>
              ))}
            </div>
          </div>
        </div>
      ) : (
        /* --- design mode: palette | canvas | inspector --------------------- */
        <div className='grid gap-4 lg:grid-cols-[11rem_minmax(0,1fr)_24rem]'>
          {/* palette */}
          <div className='rounded-xl border border-border bg-surface p-2'>
            <p className='px-1.5 pb-2 pt-1 font-mono text-xxs uppercase tracking-wider text-fg-muted'>
              inputs
            </p>
            <div className='space-y-1'>
              {TYPE_ORDER.map((id) => {
                const t = typeDef(id);
                const Icon = t.icon;
                return (
                  <button
                    key={id}
                    onClick={() => setSelectedId(draft.addParam(id))}
                    title={t.blurb}
                    className='flex w-full items-center gap-2 rounded-md border border-transparent px-2 py-1.5 text-left text-xs text-fg-muted hover:border-border hover:bg-surface-hover hover:text-fg'
                  >
                    <Icon className='h-3.5 w-3.5 shrink-0' />
                    {t.label}
                    <Plus className='ml-auto h-3 w-3 opacity-0 transition-opacity group-hover:opacity-100' />
                  </button>
                );
              })}
            </div>
          </div>

          {/* canvas — the live form, selectable */}
          <div className='rounded-xl border border-border bg-surface'>
            <div className='flex items-center gap-2 border-b border-border px-4 py-2.5'>
              <span className='font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                live preview · click a field to configure
              </span>
              <span className='ml-auto font-mono text-xxs text-fg-subtle'>
                {job.params.length} inputs
              </span>
            </div>
            <div className='space-y-1 p-3'>
              {job.params.map((spec, i) => {
                const active = spec.id === selectedId;
                return (
                  <div
                    key={spec.id}
                    onClick={() => setSelectedId(spec.id)}
                    className={cn(
                      'group relative cursor-pointer rounded-lg border p-3 transition-colors',
                      active
                        ? 'border-primary bg-primary-subtle/30'
                        : 'border-transparent hover:border-border hover:bg-surface-hover'
                    )}
                  >
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
                  Pick an input from the palette to start
                </p>
              )}
            </div>
          </div>

          {/* inspector */}
          <div className='space-y-4'>
            <div className='rounded-xl border border-border bg-surface'>
              <div className='border-b border-border px-4 py-2.5'>
                <span className='font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                  {selected ? `configure · ${selected.name}` : 'configure'}
                </span>
              </div>
              <div className='p-4'>
                {selected ? (
                  <ParamEditor
                    spec={selected}
                    onChange={(patch) => draft.updateParam(selected.id, patch)}
                    siblings={job.params.filter((p) => p.id !== selected.id).map((p) => p.name)}
                  />
                ) : (
                  <p className='py-8 text-center text-sm text-fg-subtle'>
                    Select a field on the canvas
                  </p>
                )}
              </div>
            </div>
            <YamlPane job={job} focusId={selectedId} className='max-h-[28rem]' />
            <button className='flex w-full items-center justify-center gap-2 rounded-md bg-primary px-4 py-2.5 text-sm font-medium text-fg-onPrimary hover:bg-primary-hover'>
              <Terminal className='h-4 w-4' /> Open PR against {job.source.repo}
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
