import { useParams, Link } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, GitBranch, PenLine, Save } from 'lucide-react';
import { getJob, listBuilds, triggerBuild, updateJob } from '@/services/jobDefService';
import { ApiError } from '@/services/api';
import { BuildWorkbench } from '@/features/jobs/BuildWorkbench';
import { Button, Spinner } from '@/components/ui';
import { cn } from '@/lib/cn';
import type { Build, ParameterField, ParameterValues } from '@/types/jobs';

function relativeTime(ts: number): string {
  const s = Math.round((Date.now() - ts) / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
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

function BuildHistory({ builds }: { builds: Build[] }) {
  return (
    <div className='mt-6 rounded-xl border border-border bg-surface'>
      <div className='border-b border-border px-4 py-3'>
        <h2 className='font-mono text-xs uppercase tracking-wider text-fg-muted'>Build history</h2>
      </div>
      <div className='px-4 py-1'>
        {builds.map((b) => (
          <div
            key={b.id}
            className='flex flex-wrap items-center gap-3 border-b border-border-subtle py-2.5 text-sm last:border-b-0'
          >
            <span
              className={cn(
                'rounded px-1.5 py-0.5 font-mono text-xxs uppercase tracking-wide',
                BUILD_TONE[b.status]
              )}
            >
              {b.status === 'running' && <span className='mr-1 animate-pulse'>●</span>}
              {b.status}
            </span>
            <span className='font-mono text-xs text-fg-muted'>#{b.id}</span>
            {b.environment && (
              <span className='rounded bg-surface-hover px-1.5 py-0.5 font-mono text-xxs text-fg-muted'>
                {b.environment}
              </span>
            )}
            <span className='ml-auto font-mono text-xxs text-fg-subtle'>
              {relativeTime(b.startedAt)} · {formatDuration(b.durationMs)}
            </span>
          </div>
        ))}
      </div>
    </div>
  );
}

export default function JobDetail() {
  const { slug = '' } = useParams();
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
    // The authored schema goes with the values: the server validates against
    // what the user actually saw, so an added parameter is applied rather than
    // silently dropped.
    mutationFn: ({ values, fields }: { values: ParameterValues; fields: ParameterField[] }) =>
      triggerBuild(slug, values, fields),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['builds', slug] });
      queryClient.invalidateQueries({ queryKey: ['job', slug] });
    },
  });

  const saveEdit = useMutation({
    mutationFn: (parameters: ParameterField[]) =>
      updateJob(slug, {
        name: job?.name ?? '',
        description: job?.description ?? '',
        queue: job?.queue ?? 'default',
        runtime: job?.runtime ?? '',
        command: job?.command ?? '',
        sourceRepo: job?.source.repo || undefined,
        labelSelector: job?.labelSelector,
        parameters,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['job', slug] });
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
    },
  });

  // Only panel-authored definitions can be saved. A Git-managed one would be
  // overwritten by the next sync, so it gets the export path instead.
  const editFooter =
    job?.origin === 'panel'
      ? (parameters: ParameterField[]) => (
          <div className='flex flex-wrap items-center gap-3 border-t border-border pt-4'>
            <Button
              type='button'
              disabled={saveEdit.isPending}
              onClick={() => saveEdit.mutate(parameters)}
            >
              <Save className='mr-1.5 h-4 w-4' />
              {saveEdit.isPending ? 'Saving…' : 'Save changes'}
            </Button>
            {saveEdit.isSuccess && !saveEdit.isPending && (
              <span className='text-sm text-success'>Saved.</span>
            )}
            {saveEdit.isError && (
              <span className='text-sm text-danger-fg'>
                {(saveEdit.error as Error).message}
              </span>
            )}
          </div>
        )
      : undefined;

  const fieldErrors = trigger.error instanceof ApiError ? trigger.error.fields : undefined;
  // A field-level rejection is already rendered on the inputs; repeating the
  // summary line above them would just say "invalid parameters" twice.
  const runError =
    trigger.isError && !fieldErrors ? (trigger.error as Error).message : undefined;

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

      <div className='mb-6 flex flex-wrap items-start gap-4'>
        <div className='min-w-0'>
          <div className='flex items-center gap-3'>
            <h1 className='font-mono text-2xl font-bold tracking-tight text-fg'>{job.name}</h1>
            {job.origin === 'panel' ? (
              <span className='inline-flex items-center gap-1 rounded border border-warning/30 bg-warning-subtle px-2 py-0.5 text-xxs font-medium uppercase tracking-wide text-warning-fg'>
                <PenLine className='h-3 w-3' /> panel
              </span>
            ) : (
              <span className='inline-flex items-center gap-1 rounded border border-success/30 bg-success-subtle px-2 py-0.5 text-xxs font-medium uppercase tracking-wide text-success-fg'>
                <GitBranch className='h-3 w-3' /> git-managed
              </span>
            )}
          </div>
          <p className='mt-1.5 max-w-2xl text-sm text-fg-muted'>{job.description}</p>
        </div>
        {job.origin === 'panel' ? (
          <div className='ml-auto flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-2 font-mono text-xs text-fg-muted'>
            <PenLine className='h-3.5 w-3.5' /> authored in the panel · not in Git
          </div>
        ) : (
          <div className='ml-auto flex items-center gap-2 rounded-md border border-border bg-surface px-3 py-2 font-mono text-xs text-fg-muted'>
            <GitBranch className='h-3.5 w-3.5' />
            {job.source.repo} · <span className='text-fg'>{job.source.path}</span> ·{' '}
            <span className='text-success'>@{job.source.commit}</span>
          </div>
        )}
      </div>

      <BuildWorkbench
        job={job}
        builds={builds ?? []}
        onRun={(values, fields) => trigger.mutate({ values, fields })}
        running={trigger.isPending}
        serverErrors={fieldErrors}
        runError={runError}
        queuedBuildId={trigger.isSuccess ? trigger.data.id : undefined}
        footer={editFooter}
      />

      {builds && builds.length > 0 && <BuildHistory builds={builds} />}
    </div>
  );
}
