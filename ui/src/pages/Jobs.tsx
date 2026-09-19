import { useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { FileCode2, GitBranch, Plus, RefreshCw, Timer } from 'lucide-react';
import { listJobs, syncJobs, type SyncResult } from '@/services/jobDefService';
import { EmptyState, Spinner } from '@/components/ui';
import { cn } from '@/lib/cn';
import { ConfigDialog, GitStateDot, JobActionsMenu } from '@/features/jobs/GitState';
import type { JobDefinition } from '@/types/jobs';

function formatDuration(ms: number): string {
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  return `${m}m ${String(s % 60).padStart(2, '0')}s`;
}

function JobCard({ job }: { job: JobDefinition }) {
  const hasBuilds = job.medianDurationMs > 0 || job.successRate > 0;
  const rate = Math.round(job.successRate * 100);
  const rateTone = !hasBuilds
    ? 'text-fg-subtle'
    : rate >= 97
      ? 'text-success'
      : rate >= 90
        ? 'text-warning'
        : 'text-danger';
  return (
    // The whole card links to the job; the link is stretched over it so the
    // actions menu and the Git dot can sit on top as their own targets.
    <div className='group relative flex flex-col gap-3 rounded-xl border border-border bg-surface p-4 transition-colors hover:border-border-strong hover:bg-surface-hover'>
      <Link
        to={`/jobs/${job.slug}`}
        aria-label={job.name}
        className='absolute inset-0 rounded-xl'
      />
      <GitStateDot state={job.gitState} className='absolute -right-1.5 -top-1.5 z-10' />
      <div className='pointer-events-none flex items-start gap-3'>
        <div className='min-w-0'>
          <h3 className='truncate font-mono text-sm font-semibold text-fg'>{job.name}</h3>
          <p className='mt-1 line-clamp-2 text-xs text-fg-muted'>{job.description}</p>
        </div>
        <div className='ml-auto text-right'>
          <div className={`font-mono text-lg font-bold ${rateTone}`}>
            {hasBuilds ? `${rate}%` : '—'}
          </div>
          <div className='text-xxs text-fg-subtle'>{hasBuilds ? '30d' : 'no builds'}</div>
        </div>
        <JobActionsMenu job={job} className='pointer-events-auto -mr-1.5 -mt-0.5' />
      </div>
      <div className='pointer-events-none flex items-center gap-4 border-t border-border-subtle pt-3 font-mono text-xxs text-fg-subtle'>
        <span className='rounded bg-bg-muted px-1.5 py-0.5 text-fg-muted'>
          {job.queue}
        </span>
        <span className='truncate'>{job.runtime}</span>
        <span className='ml-auto inline-flex items-center gap-1'>
          <Timer className='h-3 w-3' />{' '}
          {job.medianDurationMs > 0 ? formatDuration(job.medianDurationMs) : '—'}
        </span>
      </div>
    </div>
  );
}

function syncSummary(r: SyncResult): string {
  const parts = [
    r.added && `${r.added} added`,
    r.updated && `${r.updated} updated`,
    r.detached && `${r.detached} no longer in Git`,
    r.pruned && `${r.pruned} pruned`,
  ].filter(Boolean);
  return parts.length ? `Synced — ${parts.join(', ')}.` : 'Synced — already up to date.';
}

export default function Jobs() {
  const queryClient = useQueryClient();
  const [exporting, setExporting] = useState(false);
  const { data: jobs, isLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: listJobs,
  });

  const sync = useMutation({
    mutationFn: syncJobs,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      void queryClient.invalidateQueries({ queryKey: ['job'] });
    },
  });

  const drifted = jobs?.filter((j) => j.gitState !== 'synced').length ?? 0;
  const button =
    'inline-flex items-center gap-1.5 rounded-md border border-border bg-surface px-3 py-1.5 text-sm font-medium text-fg hover:bg-surface-hover disabled:opacity-50';

  return (
    <div>
      <div className='mb-6 flex items-end justify-between'>
        <div>
          <h1 className='text-xl font-bold tracking-tight text-fg'>Jobs</h1>
          <p className='mt-1 text-sm text-fg-muted'>
            Reusable definitions, sourced from Git. Trigger a build and watch it run.
          </p>
        </div>
        <div className='flex flex-wrap items-center justify-end gap-2'>
          <span className='mr-2 inline-flex items-center gap-1.5 font-mono text-xs text-fg-subtle'>
            {jobs ? `${jobs.length} definitions` : ''}
            {drifted > 0 && (
              <>
                {' · '}
                <span className='h-2 w-2 rounded-full bg-warning' />
                {drifted} differ from Git
              </>
            )}
          </span>
          <button
            type='button'
            className={button}
            disabled={sync.isPending}
            onClick={() => sync.mutate()}
          >
            <RefreshCw className={cn('h-4 w-4', sync.isPending && 'animate-spin')} /> Sync from Git
          </button>
          <button type='button' className={button} onClick={() => setExporting(true)}>
            <FileCode2 className='h-4 w-4' /> Export config
          </button>
          <Link to='/jobs/new' className={button}>
            <Plus className='h-4 w-4' /> New template
          </Link>
        </div>
      </div>

      {(sync.isSuccess || sync.isError) && (
        <div
          className={cn(
            'mb-4 rounded-md border px-3 py-2 text-sm',
            sync.isError
              ? 'border-danger/30 bg-danger-subtle text-danger-fg'
              : 'border-border bg-surface text-fg-muted'
          )}
        >
          {sync.isError ? (sync.error as Error).message : syncSummary(sync.data)}
          {sync.data?.skipped.map((s) => (
            <div key={s} className='mt-1 font-mono text-xs text-warning-fg'>
              skipped {s}
            </div>
          ))}
        </div>
      )}

      <ConfigDialog open={exporting} onClose={() => setExporting(false)} />

      {isLoading ? (
        <div className='flex justify-center py-20'>
          <Spinner size={28} />
        </div>
      ) : jobs && jobs.length > 0 ? (
        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
          {jobs.map((job) => <JobCard key={job.slug} job={job} />)}
        </div>
      ) : (
        <EmptyState
          icon={<GitBranch className='h-5 w-5' />}
          title='No job definitions'
          description='Job definitions are synced from Git. Add a YAML file to the job-definitions source (JOB_DEFS_DIR) and press Sync from Git — or author one from scratch with New template.'
        />
      )}
    </div>
  );
}
