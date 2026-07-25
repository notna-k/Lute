/**
 * The job build/configure surface.
 *
 * Three columns, each with a fixed role; the mode decides what fills them:
 *
 *   left    menu     — running: "start from", prefill from a previous build
 *                      editing: the configure panel for the selected parameter
 *   centre  main     — running: the form to fill in
 *                      editing: the parameter list, live, click one to configure
 *   right   context  — running: the equivalent `docker run` / `curl`
 *                      editing: the YAML you would commit, copy or download
 *
 * Editing is deliberately read-only against the server. Git is the source of
 * truth (PRODUCT.md §6), so the editor's only artifact is YAML you commit to
 * the job-definitions repo — there is no "save to panel" that would make a
 * definition expressible outside its file.
 */
import { useMemo, useState } from 'react';
import {
  Check,
  ChevronDown,
  ChevronUp,
  Copy,
  Download,
  FileCode2,
  GitCompare,
  GripVertical,
  Pencil,
  Play,
  Plus,
  Rocket,
  RotateCcw,
  Settings2,
  Terminal,
  Trash2,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { ParamField } from '@/features/params/ParamField';
import { ParamEditor } from '@/features/params/ParamEditor';
import { toEnvPairs, typeDef, valuesFromEnv } from '@/features/params/registry';
import { downloadYaml, toYaml } from '@/features/params/yaml';
import { stripIds, useSchemaDraft } from '@/features/params/useSchemaDraft';
import type { Build, JobDefinition, ParameterValues } from '@/types/jobs';

type Mode = 'run' | 'edit';
type RunPane = 'docker' | 'curl';

const STATUS_MARK: Record<Build['status'], { glyph: string; tone: string }> = {
  running: { glyph: '▸', tone: 'text-warning' },
  passed: { glyph: '✓', tone: 'text-success' },
  failed: { glyph: '✗', tone: 'text-danger' },
  queued: { glyph: '·', tone: 'text-fg-subtle' },
};

function relativeTime(ts: number): string {
  const s = Math.round((Date.now() - ts) / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

/** Minimal YAML colouring — enough to read, not a parser. */
function YamlLine({ text, highlight }: { text: string; highlight: boolean }) {
  const comment = text.trimStart().startsWith('#');
  const match = /^(\s*(?:- )?)([\w-]+)(:)(.*)$/.exec(text);
  return (
    <div className={cn('-mx-3 px-3', highlight && 'bg-primary-subtle')}>
      {comment ? (
        <span className='text-fg-subtle'>{text}</span>
      ) : match ? (
        <>
          <span>{match[1]}</span>
          <span className='text-info'>{match[2]}</span>
          <span className='text-fg-subtle'>{match[3]}</span>
          <span className='text-fg'>{match[4]}</span>
        </>
      ) : (
        <span className='text-fg'>{text || ' '}</span>
      )}
    </div>
  );
}

export interface BuildWorkbenchProps {
  job: JobDefinition;
  builds: Build[];
  onRun: (values: ParameterValues) => void;
  running: boolean;
  /** Per-field messages from the server's schema validation. */
  serverErrors?: Record<string, string>;
  runError?: string;
  queuedBuildId?: string;
}

export function BuildWorkbench({
  job,
  builds,
  onRun,
  running,
  serverErrors,
  runError,
  queuedBuildId,
}: BuildWorkbenchProps) {
  const draft = useSchemaDraft(job.parameters);
  const [mode, setMode] = useState<Mode>('run');
  const [runPane, setRunPane] = useState<RunPane>('docker');
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [focusName, setFocusName] = useState<string | null>(null);
  const [startedFrom, setStartedFrom] = useState<string | null>(null);
  const [copied, setCopied] = useState(false);
  const [dragIndex, setDragIndex] = useState<number | null>(null);
  const [overIndex, setOverIndex] = useState<number | null>(null);

  const { fields, values, setValue } = draft;
  const selected = fields.find((f) => f.id === selectedId) ?? null;
  const errors = draft.submitted ? { ...draft.localErrors, ...serverErrors } : (serverErrors ?? {});

  const doc = useMemo(() => toYaml(job, stripIds(fields)), [job, fields]);

  const payload = useMemo(() => {
    const body = Object.fromEntries(
      fields.filter((f) => f.type !== 'secret').map((f) => [f.name, values[f.name] ?? null])
    );
    return JSON.stringify({ values: body }, null, 2);
  }, [fields, values]);

  /** Builds whose stored values can seed a new run. */
  const seedable = builds.filter((b) => b.params && Object.keys(b.params).length > 0);

  function startFrom(build: Build | null) {
    if (!build) {
      draft.resetValues();
      setStartedFrom(null);
      return;
    }
    draft.setAllValues(valuesFromEnv(fields, build.params));
    setStartedFrom(build.id);
  }

  function submit() {
    draft.setSubmitted(true);
    if (!draft.valid) return;
    // Secrets resolve worker-side from their secretRef; sending a placeholder
    // would only invite the server to store one.
    const payloadValues = Object.fromEntries(
      fields.filter((f) => f.type !== 'secret').map((f) => [f.name, values[f.name]])
    );
    onRun(payloadValues);
  }

  async function copyYaml() {
    await navigator.clipboard.writeText(doc.text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }

  function addField() {
    // No type choice up front — a new parameter starts as text and is retyped
    // in the configure panel, the one place a type is chosen.
    setSelectedId(draft.addField('string'));
  }

  return (
    <div>
      {/* mode toggle */}
      <div className='mb-4 flex flex-wrap items-center gap-3'>
        <h2 className='font-mono text-xs uppercase tracking-wider text-fg-muted'>
          {mode === 'run' ? 'New build' : 'Input schema'}
        </h2>
        {mode === 'edit' && draft.dirty && (
          <span className='inline-flex items-center gap-1.5 rounded bg-warning-subtle px-2 py-1 font-mono text-xxs text-warning-fg'>
            <GitCompare className='h-3 w-3' /> uncommitted — differs from @{job.source.commit}
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
                type='button'
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
          'grid items-start gap-4',
          mode === 'edit'
            ? 'xl:grid-cols-[24rem_minmax(0,1fr)_22rem]'
            : 'xl:grid-cols-[15rem_minmax(0,1fr)_22rem]'
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
                  type='button'
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

                {seedable.length > 0 && (
                  <p className='px-2.5 pb-1 pt-2 font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
                    previous builds
                  </p>
                )}
                {seedable.map((b) => {
                  const mark = STATUS_MARK[b.status];
                  return (
                    <button
                      key={b.id}
                      type='button'
                      onClick={() => startFrom(b)}
                      title={`Fill the form with #${b.id}'s values`}
                      className={cn(
                        'flex w-full items-start gap-2 rounded-md border px-2.5 py-1.5 text-left transition-colors',
                        startedFrom === b.id
                          ? 'border-primary bg-primary-subtle'
                          : 'border-transparent hover:border-border'
                      )}
                    >
                      <span className={cn('font-mono text-xs leading-5', mark.tone)}>
                        {mark.glyph}
                      </span>
                      <span className='min-w-0 flex-1'>
                        <span className='block font-mono text-xs text-fg-muted'>#{b.id}</span>
                        {b.environment && (
                          <span className='block truncate text-xxs text-fg-subtle'>
                            {b.environment}
                          </span>
                        )}
                        <span className='block font-mono text-xxs text-fg-subtle'>
                          {relativeTime(b.startedAt)}
                        </span>
                      </span>
                    </button>
                  );
                })}
                {seedable.length === 0 && (
                  <p className='px-2.5 py-3 text-xxs leading-relaxed text-fg-subtle'>
                    No previous builds yet. Once this job has run, its values show up here as
                    starting points.
                  </p>
                )}
              </div>
            </div>
          ) : (
            /* The configure panel. No inner scroll container on purpose: one
               would trap the date picker and other popovers in a short window.
               The panel grows to its content and the page scrolls instead. */
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
                    field={selected}
                    onChange={(patch) => draft.updateField(selected.id, patch)}
                    siblings={fields.filter((f) => f.id !== selected.id).map((f) => f.name)}
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
                {fields.map((field) => (
                  <ParamField
                    key={field.id}
                    field={field}
                    value={values[field.name]}
                    onChange={(v) => setValue(field.name, v)}
                    error={errors[field.name]}
                    onFocusCapture={() => setFocusName(field.name)}
                    className='border-b border-dashed border-border py-4 last:border-b-0'
                  />
                ))}
                {fields.length === 0 && (
                  <p className='py-8 text-center text-sm text-fg-subtle'>
                    This job takes no parameters.
                  </p>
                )}
              </div>

              <div className='sticky bottom-0 z-10 mt-3 flex flex-wrap items-center gap-3 rounded-xl border border-border bg-bg-elevated px-4 py-3 shadow-card'>
                <button
                  type='button'
                  onClick={submit}
                  disabled={running}
                  className='inline-flex items-center gap-2 rounded-md bg-primary px-5 py-2.5 text-sm font-medium text-fg-onPrimary transition-colors hover:bg-primary-hover disabled:opacity-60'
                >
                  <Play className='h-4 w-4' /> {running ? 'Running…' : 'Run build'}
                </button>
                <span className='font-mono text-xs text-fg-subtle'>
                  queue <span className='text-fg-muted'>{job.queue}</span> · {job.runtime}
                </span>
                {startedFrom && (
                  <span className='rounded bg-surface-hover px-2 py-1 font-mono text-xxs text-fg-muted'>
                    started from #{startedFrom}
                  </span>
                )}
                {queuedBuildId && (
                  <span className='rounded bg-success-subtle px-2 py-1 font-mono text-xs text-success-fg'>
                    queued #{queuedBuildId}
                  </span>
                )}
                <button
                  type='button'
                  onClick={() => startFrom(null)}
                  className='ml-auto inline-flex items-center gap-1.5 font-mono text-xxs text-fg-subtle hover:text-fg'
                >
                  <RotateCcw className='h-3 w-3' /> reset
                </button>
              </div>

              {(runError || (draft.submitted && !draft.valid)) && (
                <p className='mt-2 font-mono text-xs text-danger'>
                  {runError ??
                    `${Object.keys(draft.localErrors).length} field(s) need attention`}
                </p>
              )}
            </>
          ) : (
            <div className='rounded-xl border border-border bg-surface'>
              <div className='flex items-center gap-2 border-b border-border px-4 py-2.5'>
                <span className='font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                  parameters · click one to configure
                </span>
                <span className='ml-auto font-mono text-xxs text-fg-subtle'>{fields.length}</span>
              </div>

              <div className='space-y-1 p-3'>
                {fields.map((field, i) => {
                  const active = field.id === selectedId;
                  const Icon = typeDef(field.type).icon;
                  return (
                    <div
                      key={field.id}
                      onClick={() => setSelectedId(field.id)}
                      onDragOver={(e) => {
                        e.preventDefault();
                        setOverIndex(i);
                      }}
                      onDrop={() => {
                        if (dragIndex !== null) draft.reorderField(dragIndex, i);
                        setDragIndex(null);
                        setOverIndex(null);
                      }}
                      className={cn(
                        'group relative cursor-pointer rounded-lg border p-3 pl-8 transition-colors',
                        active
                          ? 'border-primary bg-primary-subtle/25'
                          : 'border-transparent hover:border-border hover:bg-surface-hover',
                        dragIndex === i && 'opacity-40',
                        overIndex === i && dragIndex !== null && dragIndex !== i && 'border-primary'
                      )}
                    >
                      {/* Only the grip is draggable, so text selection inside
                          the controls keeps working. */}
                      <span
                        draggable
                        onDragStart={() => setDragIndex(i)}
                        onDragEnd={() => {
                          setDragIndex(null);
                          setOverIndex(null);
                        }}
                        title='Drag to reorder'
                        className='absolute left-1.5 top-3.5 cursor-grab text-fg-subtle opacity-0 transition-opacity group-hover:opacity-100'
                      >
                        <GripVertical className='h-4 w-4' />
                      </span>

                      <ParamField
                        field={field}
                        value={values[field.name]}
                        onChange={(v) => setValue(field.name, v)}
                      />

                      <div
                        className={cn(
                          'absolute right-2 top-2 flex items-center gap-0.5 rounded border border-border bg-bg-elevated p-0.5 text-fg-subtle transition-opacity',
                          active ? 'opacity-100' : 'opacity-0 group-hover:opacity-100'
                        )}
                      >
                        <span className='px-1'>
                          <Icon className='h-3.5 w-3.5' />
                        </span>
                        <button
                          type='button'
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.moveField(field.id, -1);
                          }}
                          disabled={i === 0}
                          className='rounded p-1 hover:bg-surface-hover hover:text-fg disabled:opacity-30'
                          aria-label='Move up'
                        >
                          <ChevronUp className='h-3.5 w-3.5' />
                        </button>
                        <button
                          type='button'
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.moveField(field.id, 1);
                          }}
                          disabled={i === fields.length - 1}
                          className='rounded p-1 hover:bg-surface-hover hover:text-fg disabled:opacity-30'
                          aria-label='Move down'
                        >
                          <ChevronDown className='h-3.5 w-3.5' />
                        </button>
                        <button
                          type='button'
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.duplicateField(field.id);
                          }}
                          className='rounded p-1 hover:bg-surface-hover hover:text-fg'
                          aria-label='Duplicate'
                        >
                          <Copy className='h-3.5 w-3.5' />
                        </button>
                        <button
                          type='button'
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.removeField(field.id);
                            if (selectedId === field.id) setSelectedId(null);
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

                {fields.length === 0 && (
                  <p className='rounded-lg border border-dashed border-border py-10 text-center text-sm text-fg-subtle'>
                    No parameters yet
                  </p>
                )}

                <button
                  type='button'
                  onClick={addField}
                  className='flex w-full items-center justify-center gap-2 rounded-lg border border-dashed border-border py-3 text-sm text-fg-muted transition-colors hover:border-primary hover:text-primary'
                >
                  <Plus className='h-4 w-4' /> Add input
                </button>
              </div>
            </div>
          )}
        </main>

        {/* ============================ RIGHT =========================== */}
        <aside className='min-w-0'>
          {mode === 'edit' ? (
            <div className='sticky top-4'>
              <div className='rounded-xl border border-border bg-bg-subtle'>
                <div className='flex items-center gap-2 border-b border-border px-3 py-2'>
                  <FileCode2 className='h-3.5 w-3.5 text-primary' />
                  <span className='truncate font-mono text-xxs text-fg-muted'>
                    {job.source.path}
                  </span>
                  <div className='ml-auto flex shrink-0 items-center gap-1.5'>
                    <button
                      type='button'
                      onClick={copyYaml}
                      className='inline-flex items-center gap-1.5 rounded border border-border px-2 py-1 font-mono text-xxs text-fg-muted hover:border-border-strong hover:text-fg'
                    >
                      {copied ? (
                        <Check className='h-3 w-3 text-success' />
                      ) : (
                        <Copy className='h-3 w-3' />
                      )}
                      {copied ? 'copied' : 'copy'}
                    </button>
                    <button
                      type='button'
                      onClick={() => downloadYaml(doc.text, `${job.slug}.yaml`)}
                      title={`Download ${job.slug}.yaml`}
                      className='inline-flex items-center gap-1.5 rounded border border-border px-2 py-1 font-mono text-xxs text-fg-muted hover:border-border-strong hover:text-fg'
                    >
                      <Download className='h-3 w-3' /> .yaml
                    </button>
                  </div>
                </div>
                <pre className='scrollbar-thin max-h-[32rem] overflow-auto px-3 py-2.5 font-mono text-xs leading-[1.6]'>
                  {doc.text.split('\n').map((line, i) => {
                    const range = selected ? doc.ranges[selected.name] : undefined;
                    return (
                      <YamlLine
                        key={i}
                        text={line}
                        highlight={Boolean(range && i >= range[0] && i <= range[1])}
                      />
                    );
                  })}
                </pre>
              </div>

              <p className='mt-2 px-1 text-xxs leading-relaxed text-fg-subtle'>
                Git is the source of truth. Commit this file to{' '}
                <span className='font-mono text-fg-muted'>{job.source.repo}</span> and Core picks
                the change up on its next sync — the panel never writes definitions.
              </p>
              {draft.dirty && (
                <button
                  type='button'
                  onClick={() => {
                    draft.resetFields();
                    setSelectedId(null);
                  }}
                  className='mt-2 inline-flex items-center gap-1.5 rounded border border-border px-2 py-1 font-mono text-xxs text-fg-muted hover:border-danger hover:text-danger'
                >
                  <RotateCcw className='h-3 w-3' /> discard changes
                </button>
              )}
            </div>
          ) : (
            <div className='sticky top-4'>
              <div className='mb-2 flex overflow-hidden rounded-md border border-border'>
                {[
                  { id: 'docker' as const, label: 'docker run', icon: Terminal },
                  { id: 'curl' as const, label: 'curl', icon: FileCode2 },
                ].map((p) => {
                  const Icon = p.icon;
                  return (
                    <button
                      key={p.id}
                      type='button'
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

              {runPane === 'docker' ? (
                <>
                  <div className='rounded-xl border border-border bg-bg-subtle'>
                    <div className='border-b border-border px-3 py-2 font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                      resolved environment
                    </div>
                    <div className='scrollbar-thin overflow-x-auto px-3 py-2.5 font-mono text-xs leading-relaxed'>
                      <div className='text-fg-subtle'>docker run --rm \</div>
                      {toEnvPairs(fields, values).map((p, i) => (
                        <div
                          key={p.key}
                          className={cn(
                            '-mx-1 whitespace-nowrap px-1',
                            fields[i]?.name === focusName && 'rounded bg-primary-subtle'
                          )}
                        >
                          <span className='text-fg-subtle'> -e </span>
                          <span className='text-info'>{p.key}</span>
                          <span className='text-fg-subtle'>=</span>
                          {p.masked ? (
                            <span className='text-warning'>$(secret)</span>
                          ) : p.value === '' ? (
                            <span className='italic text-fg-subtle'>&apos;&apos;</span>
                          ) : (
                            <span className='text-success'>{JSON.stringify(p.value)}</span>
                          )}
                          <span className='text-fg-subtle'> \</span>
                        </div>
                      ))}
                      <div>
                        {'  '}
                        <span className='text-fg-subtle'>{job.runtime}</span>{' '}
                        <span className='text-fg'>{job.command}</span>
                      </div>
                    </div>
                  </div>
                  <p className='mt-2 px-1 text-xxs leading-relaxed text-fg-subtle'>
                    What the worker exports before running{' '}
                    <code className='font-mono text-fg-muted'>{job.command}</code>. Secrets
                    resolve on the worker and never leave the store.
                  </p>
                </>
              ) : (
                <div className='rounded-xl border border-border bg-bg-subtle'>
                  <div className='border-b border-border px-3 py-2 font-mono text-xxs uppercase tracking-wider text-fg-muted'>
                    equivalent request
                  </div>
                  <pre className='scrollbar-thin max-h-[28rem] overflow-auto px-3 py-2.5 font-mono text-xs leading-relaxed text-fg'>
{`curl -X POST \\
  $CORE/api/v1/job-definitions/${job.slug}/trigger \\
  -H 'Authorization: Bearer $TOKEN' \\
  -d '${payload}'`}
                  </pre>
                  <p className='px-3 pb-3 text-xxs leading-relaxed text-fg-subtle'>
                    The panel and this call share one schema — anything the form rejects, the
                    API rejects.
                  </p>
                </div>
              )}
            </div>
          )}
        </aside>
      </div>
    </div>
  );
}

export default BuildWorkbench;
