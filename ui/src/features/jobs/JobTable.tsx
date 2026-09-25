import { useState } from 'react';
import { Link } from 'react-router-dom';
import { ChevronRight, FolderOpen, Play } from 'lucide-react';
import { LinkButton } from '@/components/ui/Button';
import { RowLink, TBody, Table, Td, Th, THead, Tr } from '@/components/ui/Table';
import { StatusText } from '@/components/ui/Status';
import { Tape } from '@/components/ui/Tape';
import { cn } from '@/lib/cn';
import { duration, percent, relativeTime } from '@/lib/format';
import type { BuildStatus, JobDefinition } from '@/types/jobs';
import { GitStateBadge, JobActionsMenu } from './GitState';
import type { JobGroup } from './jobFilters';

/** The job list, one collapsible section per folder. */
export function JobTable({ groups }: { groups: JobGroup[] }) {
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  return (
    <Table>
      <THead>
        <Tr>
          <Th>Job</Th>
          <Th>Last build</Th>
          <Th>When</Th>
          <Th>Duration</Th>
          <Th>History</Th>
          <Th>30d</Th>
          <Th className='text-right'>Run</Th>
        </Tr>
      </THead>
      {groups.map((group) => {
        const isCollapsed = collapsed[group.folder] ?? false;
        return (
          <TBody key={group.folder}>
            <Tr>
              {/* The folder header doubles as the collapse control: one
                  row per folder rather than a separate tree pane. */}
              <Td colSpan={7} className='border-b-border bg-bg-subtle py-1.5'>
                <button
                  type='button'
                  onClick={() =>
                    setCollapsed((prev) => ({
                      ...prev,
                      [group.folder]: !isCollapsed,
                    }))
                  }
                  aria-expanded={!isCollapsed}
                  className='inline-flex items-center gap-1.5 font-mono text-[11.5px] text-fg-muted hover:text-fg'
                >
                  <ChevronRight
                    className={cn('h-3.5 w-3.5 transition-transform', !isCollapsed && 'rotate-90')}
                  />
                  <FolderOpen className='h-3.5 w-3.5 text-fg-subtle' />
                  {group.folder}
                  <span className='text-fg-subtle'>{group.rows.length}</span>
                </button>
              </Td>
            </Tr>
            {!isCollapsed && group.rows.map((job) => <JobRow key={job.slug} job={job} />)}
          </TBody>
        );
      })}
    </Table>
  );
}

function RateCell({ job }: { job: JobDefinition }) {
  const hasBuilds = job.medianDurationMs > 0 || job.successRate > 0;
  if (!hasBuilds) return <span className='text-fg-subtle'>—</span>;
  const rate = job.successRate;
  return (
    <span
      className={rate >= 0.97 ? 'text-fg' : rate >= 0.9 ? 'text-warning' : 'text-danger'}
      title='Success rate over the trailing 30 days'
    >
      {percent(rate)}
    </span>
  );
}

function JobRow({ job }: { job: JobDefinition }) {
  const last = job.lastBuild;
  return (
    <RowLink to={`/jobs/${job.slug}`}>
      <Td className='pl-12'>
        <div className='flex items-center gap-2'>
          <Link
            to={`/jobs/${job.slug}`}
            className='truncate font-mono font-medium text-fg hover:underline'
          >
            {job.name}
          </Link>
          {job.gitState !== 'synced' && <GitStateBadge state={job.gitState} />}
        </div>
        {job.description && <p className='row-subtext max-w-[46ch]'>{job.description}</p>}
      </Td>
      <Td>
        {last ? (
          <StatusText state={last.status}>
            <span className='font-mono'>#{last.id}</span>
          </StatusText>
        ) : (
          <span className='text-fg-subtle'>never run</span>
        )}
      </Td>
      <Td className='tabular-nums text-fg-muted'>{last ? relativeTime(last.startedAt) : '—'}</Td>
      <Td className='tabular-nums text-fg-muted'>
        {duration(last?.durationMs ?? (job.medianDurationMs || null))}
      </Td>
      <Td>
        {job.recent?.length ? (
          <Tape states={job.recent as BuildStatus[]} />
        ) : (
          <span className='text-fg-subtle'>—</span>
        )}
      </Td>
      <Td className='tabular-nums'>
        <RateCell job={job} />
      </Td>
      <Td className='text-right'>
        <div className='flex items-center justify-end gap-1'>
          <LinkButton to={`/jobs/${job.slug}/run`} variant='outline' size='xs'>
            <Play className='h-3 w-3' /> Run
          </LinkButton>
          <JobActionsMenu job={job} />
        </div>
      </Td>
    </RowLink>
  );
}
