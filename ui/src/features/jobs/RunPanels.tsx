// The workbench's run-mode columns: where to start from, the form, and the equivalent invocation.
import { useMemo, useState, type ReactNode } from 'react';
import { Check, Play, RotateCcw } from 'lucide-react';
import { cn } from '@/lib/cn';
import { ParamField } from '@/features/params/ParamField';
import { toEnvPairs } from '@/features/params/registry';
import type { DraftField } from '@/features/params/useSchemaDraft';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Card, CardHeader, CardTitle } from '@/components/ui/Card';
import { Chip } from '@/components/ui/Chip';
import { SegmentedControl } from '@/components/ui/SegmentedControl';
import { StatusMark } from '@/components/ui/Status';
import { relativeTime } from '@/lib/format';
import type { Build, JobDefinition, ParameterValue, ParameterValues } from '@/types/jobs';

export function StartFromCard({
  builds,
  startedFrom,
  onStartFrom,
}: {
  builds: Build[];
  startedFrom: string | null;
  onStartFrom: (build: Build | null) => void;
}) {
  const seedable = builds.filter((b) => b.params && Object.keys(b.params).length > 0);
  return (
    <Card>
      <CardHeader>
        <CardTitle>Start from</CardTitle>
      </CardHeader>
      <div className='flex flex-col items-stretch gap-1 p-2'>
        <Chip
          selected={startedFrom === null}
          onClick={() => onStartFrom(null)}
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
            onClick={() => onStartFrom(b)}
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
  );
}

export function RunForm({
  job,
  fields,
  values,
  errors,
  onChange,
  onFocusField,
  driftNote,
  startedFrom,
  onSubmit,
  onReset,
  running,
  error,
}: {
  job: JobDefinition;
  fields: DraftField[];
  values: ParameterValues;
  errors: Record<string, string>;
  onChange: (name: string, value: ParameterValue) => void;
  onFocusField: (name: string) => void;
  driftNote: ReactNode;
  startedFrom: string | null;
  onSubmit: () => void;
  onReset: () => void;
  running: boolean;
  error?: string;
}) {
  return (
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
              onChange={(v) => onChange(field.name, v)}
              error={errors[field.name]}
              onFocusCapture={() => onFocusField(field.name)}
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
        <Button variant='primary' onClick={onSubmit} disabled={running}>
          <Play className='h-3.5 w-3.5' />
          {running ? 'Queueing…' : 'Run build'}
        </Button>
        <span className='font-mono text-[11.5px] text-fg-subtle'>
          queue <span className='text-fg-muted'>{job.queue}</span> · {job.runtime}
        </span>
        {startedFrom && <Badge size='sm'>from #{startedFrom}</Badge>}
        <Button variant='ghost' size='xs' className='ml-auto' onClick={onReset}>
          <RotateCcw className='h-3 w-3' /> reset
        </Button>
      </div>

      {error && <p className='mt-2 font-mono text-xs text-danger'>{error}</p>}
    </>
  );
}

type RunPane = 'docker' | 'curl';

/** The `docker run` or `curl` that the form amounts to. */
export function InvocationPanel({
  job,
  fields,
  values,
  focusName,
}: {
  job: JobDefinition;
  fields: DraftField[];
  values: ParameterValues;
  focusName: string | null;
}) {
  const [pane, setPane] = useState<RunPane>('docker');

  const payload = useMemo(() => {
    const body = Object.fromEntries(
      fields.filter((f) => f.type !== 'secret').map((f) => [f.name, values[f.name] ?? null]),
    );
    return JSON.stringify({ values: body }, null, 2);
  }, [fields, values]);

  return (
    <div className='sticky top-0'>
      <SegmentedControl<RunPane>
        label='Equivalent invocation'
        value={pane}
        onChange={setPane}
        className='mb-2'
        options={[
          { value: 'docker', label: 'docker run' },
          { value: 'curl', label: 'curl' },
        ]}
      />

      {pane === 'docker' ? (
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
            <code className='font-mono text-fg-muted'>{job.command}</code>. Secrets resolve on the
            worker and never leave the store.
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
            The panel and this call share one schema — anything the form rejects, the API rejects.
          </p>
        </Card>
      )}
    </div>
  );
}
