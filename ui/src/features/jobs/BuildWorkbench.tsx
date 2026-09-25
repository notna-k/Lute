// The job run/author surface: three columns whose content depends on the caller-owned mode.
import { useMemo, useState, type ReactNode } from 'react';
import { GitCompare } from 'lucide-react';
import { cn } from '@/lib/cn';
import { valuesFromEnv } from '@/features/params/registry';
import { toYaml } from '@/features/params/yaml';
import { stripIds, useSchemaDraft } from '@/features/params/useSchemaDraft';
import { Badge } from '@/components/ui/Badge';
import type { Build, JobDefinition, ParameterField, ParameterValues } from '@/types/jobs';
import { InvocationPanel, RunForm, StartFromCard } from './RunPanels';
import { ConfigurePanel, SchemaList, YamlPanel } from './SchemaPanels';

interface BuildWorkbenchProps {
  job: JobDefinition;
  builds: Build[];
  /** `run` fills in and triggers; `edit` authors the input schema. */
  mode: 'run' | 'edit';
  /** `fields` is the schema as authored, which the server validates against. */
  onRun?: (values: ParameterValues, fields: ParameterField[]) => void;
  /** The caller's save/cancel controls, rendered under the editor. */
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
  const [selectedId, setSelectedId] = useState<string | null>(null);
  const [focusName, setFocusName] = useState<string | null>(null);
  const [startedFrom, setStartedFrom] = useState<string | null>(null);

  const { fields, values } = draft;
  const selected = fields.find((f) => f.id === selectedId) ?? null;
  const errors = draft.submitted ? { ...draft.localErrors, ...serverErrors } : (serverErrors ?? {});
  const doc = useMemo(() => toYaml(job, stripIds(fields)), [job, fields]);

  function startFrom(build: Build | null) {
    if (build) draft.setAllValues(valuesFromEnv(fields, build.params));
    else draft.resetValues();
    setStartedFrom(build?.id ?? null);
  }

  function submit() {
    draft.setSubmitted(true);
    if (!draft.valid) return;
    // Secrets resolve worker-side from their secretRef, so no placeholder is sent.
    const payload = Object.fromEntries(
      fields.filter((f) => f.type !== 'secret').map((f) => [f.name, values[f.name]]),
    );
    onRun?.(payload, stripIds(fields));
  }

  const driftNote = draft.dirty && (
    <Badge tone='warning' size='sm'>
      <GitCompare className='h-3 w-3' />
      {job.source.commit ? `differs from @${job.source.commit}` : 'not committed to Git'}
    </Badge>
  );

  const invalidNote =
    draft.submitted && !draft.valid
      ? `${Object.keys(draft.localErrors).length} field(s) need attention`
      : undefined;

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
        <aside className='min-w-0'>
          {mode === 'run' ? (
            <StartFromCard builds={builds} startedFrom={startedFrom} onStartFrom={startFrom} />
          ) : (
            <ConfigurePanel draft={draft} selected={selected} />
          )}
        </aside>

        <main className='min-w-0'>
          {mode === 'run' ? (
            <RunForm
              job={job}
              fields={fields}
              values={values}
              errors={errors}
              onChange={draft.setValue}
              onFocusField={setFocusName}
              driftNote={driftNote}
              startedFrom={startedFrom}
              onSubmit={submit}
              onReset={() => startFrom(null)}
              running={running}
              error={runError ?? invalidNote}
            />
          ) : (
            <SchemaList
              draft={draft}
              selectedId={selectedId}
              onSelect={setSelectedId}
              driftNote={driftNote}
            />
          )}
        </main>

        <aside className='min-w-0'>
          {mode === 'edit' ? (
            <YamlPanel
              job={job}
              doc={doc}
              selectedName={selected?.name}
              dirty={draft.dirty}
              onDiscard={() => {
                draft.resetFields();
                setSelectedId(null);
              }}
            />
          ) : (
            <InvocationPanel job={job} fields={fields} values={values} focusName={focusName} />
          )}
        </aside>
      </div>

      {footer && mode === 'edit' && <div className='mt-6'>{footer(stripIds(fields))}</div>}
    </div>
  );
}
