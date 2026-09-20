/**
 * One build: its facts, the values it ran with, and its log.
 *
 * The facts sit in a short strip above the log rather than a card beside it —
 * the log is the reason the page exists, so everything else is a caption.
 */
import { Download, RotateCcw } from 'lucide-react';
import {
  Badge,
  Button,
  EmptyState,
  KeyValueList,
  ProgressTrack,
  StatusBadge,
  type KeyValueRow,
} from '@/components/ui';
import { LogViewer } from './LogViewer';
import { useJobLogs } from '@/hooks/useJobLogs';
import { duration, relativeTime, timestamp } from '@/lib/format';
import type { Build, JobDefinition } from '@/types/jobs';

export interface BuildPaneProps {
  job: JobDefinition;
  build?: Build;
  /** Fills the run form from this build's values. */
  onRerun?: (build: Build) => void;
}

export function BuildPane({ job, build, onRerun }: BuildPaneProps) {
  const live = build?.status === 'running' || build?.status === 'queued';
  const logs = useJobLogs(build?.runId ?? build?.id, { live });

  if (!build) {
    return (
      <div className='flex min-h-0 flex-1 items-center justify-center'>
        <EmptyState
          title='No build selected'
          description='Pick a build on the left, or run one to see its output here.'
        />
      </div>
    );
  }

  const paramRows: KeyValueRow[] = Object.entries(build.params ?? {}).map(
    ([key, value]) => ({ key, value, title: value })
  );

  // Only meaningful once the job has a track record to compare against.
  const pace =
    live && job.medianDurationMs > 0
      ? (Date.now() - build.startedAt) / job.medianDurationMs
      : null;

  return (
    <div className='flex min-h-0 flex-1 flex-col'>
      <div className='shrink-0 border-b border-border px-5 py-3'>
        <div className='flex flex-wrap items-center gap-3'>
          <StatusBadge state={build.status}>
            <span className='font-mono'>#{build.id}</span>
          </StatusBadge>
          {build.environment && <Badge size='sm'>{build.environment}</Badge>}
          {build.adHoc && (
            <Badge tone='warning' size='sm' title='Ran a panel-edited schema'>
              ad-hoc
            </Badge>
          )}
          <span
            className='font-mono text-[11.5px] text-fg-subtle tabular-nums'
            title={timestamp(build.startedAt)}
          >
            {relativeTime(build.startedAt)} · {duration(build.durationMs)}
          </span>
          <div className='ml-auto flex items-center gap-1.5'>
            {onRerun && (
              <Button variant='outline' size='xs' onClick={() => onRerun(build)}>
                <RotateCcw className='h-3 w-3' /> Run again
              </Button>
            )}
            {build.runId && (
              <Button
                variant='ghost'
                size='xs'
                onClick={() =>
                  window.open(`/api/v1/jobs/${build.runId}/logs?limit=5000`, '_blank')
                }
              >
                <Download className='h-3 w-3' /> Raw log
              </Button>
            )}
          </div>
        </div>

        {pace != null && (
          <ProgressTrack
            value={pace}
            state='running'
            showTarget
            className='mt-2.5'
          />
        )}

        {paramRows.length > 0 && (
          <KeyValueList
            rows={paramRows}
            className='mt-3 max-h-24 overflow-auto scrollbar-thin'
          />
        )}
      </div>

      <LogViewer
        logs={logs}
        title={`${job.slug} · #${build.id}`}
        className='min-h-0 flex-1'
      />
    </div>
  );
}
