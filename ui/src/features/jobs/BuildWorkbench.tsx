/**
 * The job run/author surface.
 *
 * Three columns, each with a fixed role; the mode decides what fills them:
 *
 *   left    menu     — run:  "start from", prefill from a previous build
 *                      edit: the configure panel for the selected parameter
 *   centre  main     — run:  the form to fill in
 *                      edit: the parameter list, live, click one to configure
 *   right   context  — run:  the equivalent `docker run` / `curl`
 *                      edit: the YAML you would commit, copy or download
 *
 * An edited schema can be saved (the caller's footer) — the definition then
 * differs from Git until the YAML is committed — or just *run*: it goes through
 * as an ad-hoc build carrying its own schema, gated on the operator's "allow
 * ad-hoc builds" setting.
 *
 * The mode is owned by the caller, because for an existing job it is a route
 * (/run vs /config) and for a new template there is nothing to run at all.
 */
import { useMemo, useState } from 'react';
import type { ReactNode } from 'react';
import {
  Check,
  ChevronDown,
  ChevronUp,
  Copy,
  Download,
  GitCompare,
  GripVertical,
  Play,
  Plus,
  RotateCcw,
  Trash2,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { ParamField } from '@/features/params/ParamField';
import { ParamEditor } from '@/features/params/ParamEditor';
import { toEnvPairs, typeDef, valuesFromEnv } from '@/features/params/registry';
import { downloadYaml, toYaml } from '@/features/params/yaml';
import { stripIds, useSchemaDraft } from '@/features/params/useSchemaDraft';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardHeader, CardTitle } from '@/components/ui/Card';
import { Chip } from '@/components/ui/Chip';
import { IconButton } from '@/components/ui/IconButton';
import { SegmentedControl } from '@/components/ui/SegmentedControl';
import { StatusMark } from '@/components/ui/Status';
import { relativeTime } from '@/lib/format';
import type { Build, JobDefinition, ParameterField, ParameterValues } from '@/types/jobs';

export type WorkbenchMode = 'run' | 'edit';
type RunPane = 'docker' | 'curl';

/** Minimal YAML colouring — enough to read, not a parser. */
function YamlLine({ text, highlight }: { text: string; highlight: boolean }) {
  const comment = text.trimStart().startsWith('#');
  const match = /^(\s*(?:- )?)([\w-]+)(:)(.*)$/.exec(text);
  return (
    <div className={cn('-mx-3 px-3', highlight && 'bg-warning-subtle')}>
      {comment ? (
        <span className='text-fg-subtle'>{text}</span>
      ) : match ? (
        <>
          <span>{match[1]}</span>
          <span className='text-fg-muted'>{match[2]}</span>
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
  /** `run` fills in and triggers; `edit` authors the input schema. */
  mode: WorkbenchMode;
  /**
   * `fields` is the schema as currently authored — the server validates against
   * it, so a parameter added here actually reaches the build instead of being
   * dropped. Compare it with `job.parameters` to know whether this is a drifted
   * (ad-hoc) build.
   */
  onRun?: (values: ParameterValues, fields: ParameterField[]) => void;
  /**
   * Rendered under the editor — the caller's save/cancel controls. Receives the
   * schema as currently authored, since the workbench owns that state.
   */
  footer?: (fields: ParameterField[]) => ReactNode;
  running?: boolean;
  /** Per-field messages from the server's schema validation. */
  serverErrors?: Record<string, string>;
  runError?: string;
}

export function BuildWorkbench({
  job,
  builds,
  mode,
  onRun,
  running = false,
  serverErrors,
  runError,
  footer,
}: BuildWorkbenchProps) {
  const draft = useSchemaDraft(job.parameters);
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
      fields.filter((f) => f.type !== 'secret').map((f) => [f.name, values[f.name] ?? null]),
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
      fields.filter((f) => f.type !== 'secret').map((f) => [f.name, values[f.name]]),
    );
    onRun?.(payloadValues, stripIds(fields));
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

  const driftNote = draft.dirty && (
    <Badge tone='warning' size='sm'>
      <GitCompare className='h-3 w-3' />
      {job.source.commit ? `differs from @${job.source.commit}` : 'not committed to Git'}
    </Badge>
  );

  return (
    <div>
      <div
        className={cn(
          'grid items-start gap-5',
          mode === 'edit'
            ? 'xl:grid-cols-[22rem_minmax(0,1fr)_22rem]'
            : 'xl:grid-cols-[14rem_minmax(0,1fr)_22rem]',
        )}
      >
        {/* ============================ LEFT ============================ */}
        <aside className='min-w-0'>
          {mode === 'run' ? (
            <Card>
              <CardHeader>
                <CardTitle>Start from</CardTitle>
              </CardHeader>
              <div className='flex flex-col items-stretch gap-1 p-2'>
                <Chip
                  selected={startedFrom === null}
                  onClick={() => startFrom(null)}
                  className='justify-start'
                >
                  Job defaults
                  {startedFrom === null && <Check className='ml-auto h-3 w-3' />}
                </Chip>
                {seedable.length > 0 && <span className='caption px-1 pt-2'>previous builds</span>}
                {seedable.map((b) => (
                  <Chip
                    key={b.id}
                    selected={startedFrom === b.id}
                    onClick={() => startFrom(b)}
                    title={`Fill the form with #${b.id}'s values`}
                    className='justify-start'
                  >
                    <StatusMark state={b.status} size={8} />
                    <span className='font-mono'>#{b.id}</span>
                    <span className='ml-auto text-fg-subtle'>{relativeTime(b.startedAt)}</span>
                  </Chip>
                ))}
                {seedable.length === 0 && (
                  <p className='px-1 py-2 text-[11.5px] leading-relaxed text-fg-subtle'>
                    Once this job has run, its values show up here as starting points.
                  </p>
                )}
              </div>
            </Card>
          ) : (
            /* The configure panel. No inner scroll container on purpose: one
               would trap the date picker and other popovers in a short window.
               The panel grows to its content and the page scrolls instead. */
            <Card className='sticky top-0'>
              <CardHeader>
                <CardTitle>Configure{selected ? ` · ${selected.name}` : ''}</CardTitle>
              </CardHeader>
              <div className='p-4'>
                {selected ? (
                  <ParamEditor
                    field={selected}
                    onChange={(patch) => draft.updateField(selected.id, patch)}
                    siblings={fields.filter((f) => f.id !== selected.id).map((f) => f.name)}
                  />
                ) : (
                  <p className='py-8 text-center text-[12.5px] text-fg-subtle'>
                    Pick a parameter to configure it
                  </p>
                )}
              </div>
            </Card>
          )}
        </aside>

        {/* =========================== CENTER =========================== */}
        <main className='min-w-0'>
          {mode === 'run' ? (
            <>
              <Card>
                <CardHeader className='flex items-center gap-2.5'>
                  <CardTitle>Parameters</CardTitle>
                  {driftNote}
                </CardHeader>
                <div className='px-5 py-1'>
                  {fields.map((field) => (
                    <ParamField
                      key={field.id}
                      field={field}
                      value={values[field.name]}
                      onChange={(v) => setValue(field.name, v)}
                      error={errors[field.name]}
                      onFocusCapture={() => setFocusName(field.name)}
                      className='border-b border-border-subtle py-4 last:border-b-0'
                    />
                  ))}
                  {fields.length === 0 && (
                    <p className='py-8 text-center text-[12.5px] text-fg-subtle'>
                      This job takes no parameters.
                    </p>
                  )}
                </div>
              </Card>

              <div className='sticky bottom-0 z-10 mt-3 flex flex-wrap items-center gap-3 border border-border bg-bg-elevated px-4 py-3 shadow-overlay'>
                <Button variant='primary' onClick={submit} disabled={running}>
                  <Play className='h-3.5 w-3.5' />
                  {running ? 'Queueing…' : 'Run build'}
                </Button>
                <span className='font-mono text-[11.5px] text-fg-subtle'>
                  queue <span className='text-fg-muted'>{job.queue}</span> · {job.runtime}
                </span>
                {startedFrom && <Badge size='sm'>from #{startedFrom}</Badge>}
                <Button
                  variant='ghost'
                  size='xs'
                  className='ml-auto'
                  onClick={() => startFrom(null)}
                >
                  <RotateCcw className='h-3 w-3' /> reset
                </Button>
              </div>

              {(runError || (draft.submitted && !draft.valid)) && (
                <p className='mt-2 font-mono text-xs text-danger'>
                  {runError ?? `${Object.keys(draft.localErrors).length} field(s) need attention`}
                </p>
              )}
            </>
          ) : (
            <Card>
              <CardHeader className='flex items-center gap-2.5'>
                <CardTitle>Parameters · click one to configure</CardTitle>
                {driftNote}
                <span className='ml-auto font-mono text-[11.5px] text-fg-subtle'>
                  {fields.length}
                </span>
              </CardHeader>

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
                        'group relative cursor-pointer border p-3 pl-8 transition-colors',
                        active
                          ? 'border-fg bg-bg-subtle'
                          : 'border-transparent hover:border-border hover:bg-surface-hover',
                        dragIndex === i && 'opacity-40',
                        overIndex === i && dragIndex !== null && dragIndex !== i && 'border-fg',
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
                          'absolute right-2 top-2 flex items-center gap-0.5 border border-border bg-bg-elevated p-0.5 text-fg-subtle transition-opacity',
                          active ? 'opacity-100' : 'opacity-0 group-hover:opacity-100',
                        )}
                      >
                        <span className='px-1'>
                          <Icon className='h-3.5 w-3.5' />
                        </span>
                        <IconButton
                          label='Move up'
                          variant='ghost'
                          size='sm'
                          disabled={i === 0}
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.moveField(field.id, -1);
                          }}
                        >
                          <ChevronUp className='h-3.5 w-3.5' />
                        </IconButton>
                        <IconButton
                          label='Move down'
                          variant='ghost'
                          size='sm'
                          disabled={i === fields.length - 1}
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.moveField(field.id, 1);
                          }}
                        >
                          <ChevronDown className='h-3.5 w-3.5' />
                        </IconButton>
                        <IconButton
                          label='Duplicate'
                          variant='ghost'
                          size='sm'
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.duplicateField(field.id);
                          }}
                        >
                          <Copy className='h-3.5 w-3.5' />
                        </IconButton>
                        <IconButton
                          label='Remove'
                          variant='ghost'
                          size='sm'
                          onClick={(e) => {
                            e.stopPropagation();
                            draft.removeField(field.id);
                            if (selectedId === field.id) setSelectedId(null);
                          }}
                        >
                          <Trash2 className='h-3.5 w-3.5' />
                        </IconButton>
                      </div>
                    </div>
                  );
                })}

                {fields.length === 0 && (
                  <p className='border border-dashed border-border py-10 text-center text-[12.5px] text-fg-subtle'>
                    No parameters yet
                  </p>
                )}

                <button
                  type='button'
                  onClick={addField}
                  className='flex w-full items-center justify-center gap-2 border border-dashed border-border py-3 text-[12.5px] text-fg-muted transition-colors hover:border-fg hover:text-fg'
                >
                  <Plus className='h-3.5 w-3.5' /> Add input
                </button>
              </div>
            </Card>
          )}
        </main>

        {/* ============================ RIGHT =========================== */}
        <aside className='min-w-0'>
          {mode === 'edit' ? (
            <div className='sticky top-0'>
              <Card className='bg-bg-subtle'>
                <CardHeader className='flex items-center gap-2'>
                  <CardTitle className='truncate normal-case tracking-normal'>
                    {job.source.path || `${job.slug}.yaml`}
                  </CardTitle>
                  <div className='ml-auto flex shrink-0 items-center gap-1.5'>
                    <Button variant='outline' size='xs' onClick={copyYaml}>
                      {copied ? (
                        <Check className='h-3 w-3 text-success' />
                      ) : (
                        <Copy className='h-3 w-3' />
                      )}
                      {copied ? 'copied' : 'copy'}
                    </Button>
                    <Button
                      variant='outline'
                      size='xs'
                      title={`Download ${job.slug}.yaml`}
                      onClick={() => downloadYaml(doc.text, `${job.slug}.yaml`)}
                    >
                      <Download className='h-3 w-3' /> .yaml
                    </Button>
                  </div>
                </CardHeader>
                <pre className='scrollbar-thin max-h-[30rem] overflow-auto px-3 py-2.5 font-mono text-[12px] leading-[1.6]'>
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
              </Card>

              <p className='mt-2 text-[11.5px] leading-relaxed text-fg-subtle'>
                Git is the source of truth. Commit this file to{' '}
                <span className='font-mono text-fg-muted'>
                  {job.source.repo || 'the job-definitions repo'}
                </span>{' '}
                and Core picks the change up on its next sync.
              </p>
              {draft.dirty && (
                <Button
                  variant='outline'
                  size='xs'
                  className='mt-2'
                  onClick={() => {
                    draft.resetFields();
                    setSelectedId(null);
                  }}
                >
                  <RotateCcw className='h-3 w-3' /> discard changes
                </Button>
              )}
            </div>
          ) : (
            <div className='sticky top-0'>
              <SegmentedControl<RunPane>
                label='Equivalent invocation'
                value={runPane}
                onChange={setRunPane}
                className='mb-2'
                options={[
                  { value: 'docker', label: 'docker run' },
                  { value: 'curl', label: 'curl' },
                ]}
              />

              {runPane === 'docker' ? (
                <>
                  <Card className='bg-bg-subtle'>
                    <CardHeader>
                      <CardTitle>Resolved environment</CardTitle>
                    </CardHeader>
                    <div className='scrollbar-thin overflow-x-auto px-3 py-2.5 font-mono text-[12px] leading-relaxed'>
                      <div className='text-fg-subtle'>docker run --rm \</div>
                      {toEnvPairs(fields, values).map((p, i) => (
                        <div
                          key={p.key}
                          className={cn(
                            '-mx-1 whitespace-nowrap px-1',
                            fields[i]?.name === focusName && 'bg-warning-subtle',
                          )}
                        >
                          <span className='text-fg-subtle'> -e </span>
                          <span className='text-fg-muted'>{p.key}</span>
                          <span className='text-fg-subtle'>=</span>
                          {p.masked ? (
                            <span className='text-warning'>$(secret)</span>
                          ) : p.value === '' ? (
                            <span className='italic text-fg-subtle'>&apos;&apos;</span>
                          ) : (
                            <span className='text-fg'>{JSON.stringify(p.value)}</span>
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
                  </Card>
                  <p className='mt-2 text-[11.5px] leading-relaxed text-fg-subtle'>
                    What the worker exports before running{' '}
                    <code className='font-mono text-fg-muted'>{job.command}</code>. Secrets resolve
                    on the worker and never leave the store.
                  </p>
                </>
              ) : (
                <Card className='bg-bg-subtle'>
                  <CardHeader>
                    <CardTitle>Equivalent request</CardTitle>
                  </CardHeader>
                  <pre className='scrollbar-thin max-h-[26rem] overflow-auto px-3 py-2.5 font-mono text-[12px] leading-relaxed text-fg'>
                    {`curl -X POST \\
  $CORE/api/v1/job-definitions/${job.slug}/trigger \\
  -H 'Authorization: Bearer $TOKEN' \\
  -d '${payload}'`}
                  </pre>
                  <p className='px-3 pb-3 text-[11.5px] leading-relaxed text-fg-subtle'>
                    The panel and this call share one schema — anything the form rejects, the API
                    rejects.
                  </p>
                </Card>
              )}
            </div>
          )}
        </aside>
      </div>

      {footer && mode === 'edit' && <div className='mt-6'>{footer(stripIds(fields))}</div>}
    </div>
  );
}

export default BuildWorkbench;
