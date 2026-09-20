/**
 * One execution — the engine-level view of a run.
 *
 * Same shape as a build page and for the same reason: the frame stays put and
 * the log gets the remaining height. The facts live in a strip above it, because
 * the question on this page is almost always "what does the log say".
 */
import { useCallback, useEffect, useState } from 'react';
import { Link, useParams } from 'react-router-dom';
import { RefreshCw, RotateCcw, X } from 'lucide-react';
import {
  Alert,
  Badge,
  Button,
  KeyValueList,
  Skeleton,
  type BadgeTone,
  type KeyValueRow,
} from '@/components/ui';
import { DetailHeader, PageBody, PageScroll } from '@/components/layout';
import { LogViewer } from '@/features/jobs/LogViewer';
import { useJobLogs } from '@/hooks/useJobLogs';
import { jobService, type Job } from '@/services/jobService';
import { duration, relativeTime, timestamp } from '@/lib/format';

const STATUS_TONE: Record<string, BadgeTone> = {
  done: 'success',
  running: 'warning',
  pending: 'warning',
  dead: 'danger',
  cancelled: 'neutral',
};

/** The engine stamps unix seconds; every formatter here speaks milliseconds. */
const ms = (unix?: number) => (unix ? unix * 1000 : undefined);

export default function ExecutionDetail() {
  const { id } = useParams<{ id: string }>();

  const [job, setJob] = useState<Job | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [actionError, setActionError] = useState<string | null>(null);
  const [busy, setBusy] = useState<'retry' | 'cancel' | null>(null);

  const fetchJob = useCallback(async () => {
    if (!id) return;
    setLoading(true);
    setError(null);
    try {
      setJob(await jobService.getJob(id));
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load job');
    } finally {
      setLoading(false);
    }
  }, [id]);

  useEffect(() => {
    void fetchJob();
  }, [fetchJob]);

  const logs = useJobLogs(id, {
    live: job?.status === 'running' || job?.status === 'pending',
  });

  async function act(kind: 'retry' | 'cancel') {
    if (!id) return;
    setBusy(kind);
    setActionError(null);
    try {
      if (kind === 'retry') await jobService.retryJob(id);
      else await jobService.cancelJob(id);
      await fetchJob();
    } catch (e) {
      setActionError(e instanceof Error ? e.message : `${kind} failed`);
    } finally {
      setBusy(null);
    }
  }

  if (loading && !job) {
    return (
      <PageScroll>
        <PageBody className='space-y-3'>
          <Skeleton className='h-6 w-48' />
          <Skeleton className='h-40 w-full' />
        </PageBody>
      </PageScroll>
    );
  }

  if (error || !job) {
    return (
      <PageScroll>
        <PageBody>
          <Alert tone='danger' title='Execution not found'>
            {error ?? 'This run is no longer in the engine.'}{' '}
            <Link to='/executions' className='underline'>
              Back to executions
            </Link>
          </Alert>
        </PageBody>
      </PageScroll>
    );
  }

  const rows: KeyValueRow[] = [
    { key: 'queue', value: job.queue },
    { key: 'type', value: <span className='font-mono'>{job.type}</span> },
    { key: 'enqueued', value: ms(job.enqueued_at) ? timestamp(ms(job.enqueued_at)!) : '—' },
    { key: 'started', value: ms(job.started_at) ? timestamp(ms(job.started_at)!) : '—' },
    { key: 'finished', value: ms(job.done_at) ? timestamp(ms(job.done_at)!) : '—' },
    { key: 'attempts', value: `${job.attempts} / ${job.max_retries}` },
    { key: 'timeout', value: `${job.timeout_sec}s` },
    ...(job.worker_id
      ? [
          {
            key: 'worker',
            value: (
              <Link
                to={`/workers/${job.worker_id}`}
                className='font-mono hover:underline'
              >
                {job.worker_id}
              </Link>
            ),
          },
        ]
      : []),
  ];

  const elapsedMs =
    ms(job.done_at) && ms(job.started_at)
      ? ms(job.done_at)! - ms(job.started_at)!
      : undefined;

  return (
    <>
      <DetailHeader
        crumbs={[{ label: 'Builds', to: '/executions' }]}
        title={job.id}
        tags={
          <Badge tone={STATUS_TONE[job.status] ?? 'neutral'} size='sm' dot>
            {job.status}
          </Badge>
        }
        subtitle={`${job.type} · ${job.queue}`}
        actions={
          <>
            <Button
              variant='outline'
              size='sm'
              onClick={async () => {
                await fetchJob();
                logs.reload();
              }}
            >
              <RefreshCw className='h-3.5 w-3.5' /> Refresh
            </Button>
            {(job.status === 'dead' || job.status === 'done') && (
              <Button
                variant='outline'
                size='sm'
                disabled={busy === 'retry'}
                onClick={() => void act('retry')}
              >
                <RotateCcw className='h-3.5 w-3.5' /> Retry
              </Button>
            )}
            {job.status === 'pending' && (
              <Button
                variant='danger'
                size='sm'
                disabled={busy === 'cancel'}
                onClick={() => void act('cancel')}
              >
                <X className='h-3.5 w-3.5' /> Cancel
              </Button>
            )}
          </>
        }
        meta={
          <>
            {elapsedMs != null && <span className='tabular-nums'>{duration(elapsedMs)}</span>}
            {ms(job.enqueued_at) && (
              <span className='tabular-nums'>{relativeTime(ms(job.enqueued_at)!)}</span>
            )}
          </>
        }
      />

      <div className='shrink-0 border-b border-border px-7 py-3'>
        {actionError && (
          <Alert tone='danger' className='mb-3'>
            {actionError}
          </Alert>
        )}
        {job.error && (
          <Alert tone='danger' title='Error' className='mb-3'>
            <pre className='whitespace-pre-wrap break-all font-mono text-[12px]'>
              {job.error}
            </pre>
          </Alert>
        )}
        <KeyValueList
          rows={rows}
          className='max-h-28 overflow-auto scrollbar-thin md:grid-cols-[auto_minmax(0,1fr)_auto_minmax(0,1fr)]'
        />
      </div>

      <LogViewer logs={logs} title={`jobs/${job.id}.log`} className='min-h-0 flex-1' />
    </>
  );
}
