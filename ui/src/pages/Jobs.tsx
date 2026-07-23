import { useQuery } from '@tanstack/react-query';
import { Link } from 'react-router-dom';
import { GitBranch, Timer } from 'lucide-react';
import { listJobs } from '@/services/jobDefService';
import { Spinner } from '@/components/ui';
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
    <Link
      to={`/jobs/${job.slug}`}
      className='group flex flex-col gap-3 rounded-xl border border-border bg-surface p-4 transition-colors hover:border-border-strong hover:bg-surface-hover'
    >
      <div className='flex items-start gap-3'>
        <div className='min-w-0'>
          <div className='flex items-center gap-2'>
            <h3 className='truncate font-mono text-sm font-semibold text-fg'>
              {job.name}
            </h3>
            <span className='inline-flex items-center gap-1 rounded border border-success/30 bg-success-subtle px-1.5 py-0.5 text-xxs font-medium uppercase tracking-wide text-success-fg'>
              <GitBranch className='h-3 w-3' /> git
            </span>
          </div>
          <p className='mt-1 line-clamp-2 text-xs text-fg-muted'>{job.description}</p>
        </div>
        <div className='ml-auto text-right'>
          <div className={`font-mono text-lg font-bold ${rateTone}`}>
            {hasBuilds ? `${rate}%` : '—'}
          </div>
          <div className='text-xxs text-fg-subtle'>{hasBuilds ? '30d' : 'no builds'}</div>
        </div>
      </div>
      <div className='flex items-center gap-4 border-t border-border-subtle pt-3 font-mono text-xxs text-fg-subtle'>
        <span className='rounded bg-bg-muted px-1.5 py-0.5 text-fg-muted'>
          {job.queue}
        </span>
        <span className='truncate'>{job.runtime}</span>
        <span className='ml-auto inline-flex items-center gap-1'>
          <Timer className='h-3 w-3' />{' '}
          {job.medianDurationMs > 0 ? formatDuration(job.medianDurationMs) : '—'}
        </span>
      </div>
    </Link>
  );
}

export default function Jobs() {
  const { data: jobs, isLoading } = useQuery({
    queryKey: ['jobs'],
    queryFn: listJobs,
  });

  return (
    <div>
      <div className='mb-6 flex items-end justify-between'>
        <div>
          <h1 className='text-xl font-bold tracking-tight text-fg'>Jobs</h1>
          <p className='mt-1 text-sm text-fg-muted'>
            Reusable, Git-managed definitions. Trigger a build and watch it run.
          </p>
        </div>
        <span className='font-mono text-xs text-fg-subtle'>
          {jobs ? `${jobs.length} definitions` : ''}
        </span>
      </div>

      {isLoading ? (
        <div className='flex justify-center py-20'>
          <Spinner size={28} />
        </div>
      ) : (
        <div className='grid gap-4 sm:grid-cols-2 lg:grid-cols-3'>
          {jobs?.map((job) => <JobCard key={job.slug} job={job} />)}
        </div>
      )}
    </div>
  );
}
