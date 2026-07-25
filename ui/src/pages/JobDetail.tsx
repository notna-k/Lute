import { useState } from 'react';
import { useParams, Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, GitBranch, Play } from 'lucide-react';
import { getJob, listBuilds, triggerBuild } from '@/services/jobDefService';
import { ApiError } from '@/services/api';
import { ParameterForm } from '@/features/jobs/ParameterForm';
import { Spinner } from '@/components/ui';
import { cn } from '@/lib/cn';
import type { Build, JobDefinition, ParameterValues } from '@/types/jobs';

function relativeTime(ts: number): string {
  const s = Math.round((Date.now() - ts) / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  return `${h}h ago`;
}

function formatDuration(ms?: number): string {
  if (!ms) return '—';
  const s = Math.round(ms / 1000);
  return `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, '0')}s`;
}

const BUILD_TONE: Record<Build['status'], string> = {
  running: 'text-warning bg-warning-subtle',
  passed: 'text-success bg-success-subtle',
  failed: 'text-danger bg-danger-subtle',
  queued: 'text-fg-muted bg-surface-hover',
};

function toYaml(job: JobDefinition): string {
  const lines = [
    `# ${job.source.path} — source of truth`,
    `name: ${job.name}`,
    `queue: ${job.queue}`,
    `runtime: ${job.runtime}`,
    `command: ${job.command}`,
    'parameters:',
  ];
  for (const p of job.parameters) {
    lines.push(`  - name: ${p.name}`);
    lines.push(`    type: ${p.type}${p.required ? '   required: true' : ''}`);
    if (p.options) lines.push(`    options: [${p.options.map((o) => o.value).join(', ')}]`);
    if (p.default !== undefined) {
      lines.push(`    default: ${Array.isArray(p.default) ? `[${p.default.join(', ')}]` : p.default}`);
    }
  }
  return lines.join('\n');
}

function RecentBuilds({ builds }: { builds: Build[] }) {
  return (
    <div className='rounded-xl border border-border bg-surface'>
      <div className='border-b border-border px-4 py-3'>
        <h2 className='font-mono text-xs uppercase tracking-wider text-fg-muted'>
          Recent builds
        </h2>
      </div>
      <div className='px-3 py-1'>
        {builds.map((b) => (
          <div
            key={b.id}
            className='flex items-center gap-3 border-b border-border-subtle py-2.5 text-sm last:border-b-0'
          >
            <span
              className={cn(
                'rounded px-1.5 py-0.5 font-mono text-xxs uppercase tracking-wide',
                BUILD_TONE[b.status]
              )}
            >
              {b.status === 'running' && (
                <span className='mr-1 animate-pulse'>●</span>
              )}
              {b.status}
            </span>
            <span className='font-mono text-xs text-fg-muted'>#{b.id}</span>
            <div className='ml-auto text-right'>
              <div className='text-xs text-fg'>{b.environment}</div>
              <div className='font-mono text-xxs text-fg-subtle'>
                {relativeTime(b.startedAt)} · {formatDuration(b.durationMs)}
              </div>
            </div>
          </div>
        ))}
      </div>
    </div>
  );
}

function LiveLog({ running }: { running?: Build }) {
  return (
    <div className='overflow-hidden rounded-xl border border-border bg-bg'>
      <div className='flex items-center gap-2 border-b border-border px-3 py-2.5 font-mono text-xxs text-fg-subtle'>
        <span className='h-2.5 w-2.5 rounded-full bg-danger' />
        <span className='h-2.5 w-2.5 rounded-full bg-warning' />
        <span className='h-2.5 w-2.5 rounded-full bg-success' />
        <span className='ml-1'>{running ? `#${running.id} · live tail` : 'log tail'}</span>
      </div>
      <div className='scrollbar-thin flex max-h-56 min-h-[7rem] items-center justify-center overflow-auto p-4 text-center font-mono text-xs text-fg-subtle'>
        {running ? (
          <span>
            Build <span className='text-fg'>#{running.id}</span> is running.
            Streaming from the worker…
            <span className='animate-pulse text-warning'>█</span>
          </span>
        ) : (
          <span>
            No build streaming. Trigger a build — logs tail here live from the
            worker while it runs.
          </span>
        )}
      </div>
    </div>
  );
}

export default function JobDetail() {
  const { slug = '' } = useParams();
  const [view, setView] = useState<'form' | 'source'>('form');
  const [values, setValues] = useState<ParameterValues>({});
  const queryClient = useQueryClient();

  const { data: job, isLoading } = useQuery({
    queryKey: ['job', slug],
    queryFn: () => getJob(slug),
  });
  const { data: builds } = useQuery({
    queryKey: ['builds', slug],
    queryFn: () => listBuilds(slug),
    // Builds move through queued → running → passed/failed on the worker, so
    // keep polling while any of them is still in flight.
    refetchInterval: (query) =>
      query.state.data?.some((b) => b.status === 'queued' || b.status === 'running')
        ? 2000
        : 15000,
  });

  const trigger = useMutation({
    mutationFn: () => triggerBuild(slug, values),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['builds', slug] });
      queryClient.invalidateQueries({ queryKey: ['job', slug] });
    },
  });

  const fieldErrors =
    trigger.error instanceof ApiError ? trigger.error.fields : undefined;

  if (isLoading) {
    return (
      <div className='flex justify-center py-20'>
        <Spinner size={28} />
      </div>
    );
  }

  if (!job) {
    return (
      <div className='py-20 text-center'>
        <p className='text-fg-muted'>Job not found.</p>
        <Link to='/jobs' className='mt-2 inline-block text-sm text-primary hover:underline'>
          Back to jobs
        </Link>
      </div>
    );
  }

  return (
    <div>
      <Link
        to='/jobs'
        className='mb-4 inline-flex items-center gap-1.5 text-xs text-fg-muted hover:text-fg'
      >
        <ArrowLeft className='h-3.5 w-3.5' /> Jobs
      </Link>

      {/* Header */}
      <div className='mb-6 flex flex-wrap items-start gap-4'>
        <div className='min-w-0'>
          <div className='flex items-center gap-3'>
            <h1 className='font-mono text-2xl font-bold tracking-tight text-fg'>
              {job.name}
            </h1>
            <span className='inline-flex items-center gap-1 rounded border border-success/30 bg-success-subtle px-2 py-0.5 text-xxs font-medium uppercase tracking-wide text-success-fg'>
              <GitBranch className='h-3 w-3' /> git-managed
            </span>
          </div>
          <p className='mt-1.5 max-w-2xl text-sm text-fg-muted'>{job.description}</p>
        </div>
        <div className='ml-auto flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-2 font-mono text-xs text-fg-muted'>
          <GitBranch className='h-3.5 w-3.5' />
          {job.source.repo} · <span className='text-fg'>{job.source.path}</span> ·{' '}
          <span className='text-success'>@{job.source.commit}</span>
        </div>
      </div>

      <div className='grid items-start gap-6 lg:grid-cols-[minmax(0,1fr)_340px]'>
        {/* LEFT — trigger */}
        <div className='rounded-xl border border-border bg-surface'>
          <div className='flex items-center gap-3 border-b border-border px-4 py-3'>
            <h2 className='font-mono text-xs uppercase tracking-wider text-fg-muted'>
              Trigger a build
            </h2>
            <div className='ml-auto flex rounded-lg border border-border bg-bg p-0.5'>
              {(['form', 'source'] as const).map((v) => (
                <button
                  key={v}
                  type='button'
                  onClick={() => setView(v)}
                  className={cn(
                    'rounded-md px-3 py-1 font-mono text-xs transition-colors',
                    view === v
                      ? 'bg-surface-active text-fg'
                      : 'text-fg-muted hover:text-fg'
                  )}
                >
                  {v}
                </button>
              ))}
            </div>
          </div>
          <div className='px-4 py-3'>
            {view === 'form' ? (
              <ParameterForm
                fields={job.parameters}
                onChange={setValues}
                errors={fieldErrors}
              />
            ) : (
              <pre className='scrollbar-thin overflow-auto py-2 font-mono text-xs leading-relaxed text-fg-muted'>
                {toYaml(job)}
              </pre>
            )}
          </div>
        </div>

        {/* RIGHT */}
        <div className='flex flex-col gap-6'>
          {builds && builds.length > 0 && <RecentBuilds builds={builds} />}
          <LiveLog running={builds?.find((b) => b.status === 'running')} />
        </div>

        {/* Run bar — spans both columns, as in poc/console.html */}
        <div className='flex flex-wrap items-center gap-3 rounded-xl border border-border bg-gradient-to-r from-surface to-surface-hover px-4 py-3 lg:col-span-2'>
          <span className='font-mono text-xs text-fg-muted'>
            Dispatches to <span className='text-fg'>{job.queue}</span> · worker{' '}
            <span className='text-fg'>
              {Object.entries(job.labelSelector).map(([k, v]) => `${k}=${v}`).join(', ') || 'any'}
            </span>
          </span>
          {trigger.isError && (
            <span className='font-mono text-xs text-danger'>
              {(trigger.error as Error).message}
            </span>
          )}
          {trigger.isSuccess && (
            <span className='font-mono text-xs text-success'>
              Build #{trigger.data.id} queued
            </span>
          )}
          <button
            type='button'
            onClick={() => trigger.mutate()}
            disabled={trigger.isPending}
            className='ml-auto inline-flex items-center gap-2 rounded-md bg-primary px-5 py-2 text-sm font-semibold text-fg-onPrimary shadow-sm transition-colors hover:bg-primary-hover disabled:opacity-60'
          >
            <Play className='h-4 w-4' /> {trigger.isPending ? 'Running…' : 'Run build'}
          </button>
        </div>
      </div>
    </div>
  );
}
