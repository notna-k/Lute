// The worker fleet as a table, with re-enable and delete inline on each row.
import type { ReactNode } from 'react';
import { Link } from 'react-router-dom';
import { Power, Trash2 } from 'lucide-react';
import type { Worker } from '@/types';
import { Button } from '@/components/ui/Button';
import { IconButton } from '@/components/ui/IconButton';
import { LabelChips } from '@/components/ui/KeyValueList';
import { Meter } from '@/components/ui/Meter';
import { RowLink, TBody, Table, Td, Th, THead, Tr } from '@/components/ui/Table';
import { Skeleton } from '@/components/ui/Skeleton';
import { StatusText } from '@/components/ui/Status';
import { relativeTime, toEpochMs } from '@/lib/format';
import { metric, workerState } from './utils';

export interface WorkerListProps {
  workers: Worker[];
  loading?: boolean;
  onReEnable?: (w: Worker) => void;
  onDelete?: (w: Worker) => void;
  reEnablingId?: string;
  deletingId?: string;
  empty: ReactNode;
}

export function WorkerList({
  workers,
  loading,
  onReEnable,
  onDelete,
  reEnablingId,
  deletingId,
  empty,
}: WorkerListProps) {
  if (loading) {
    return (
      <div className='space-y-2 px-7 py-6'>
        {Array.from({ length: 5 }).map((_, i) => (
          <Skeleton key={i} className='h-9 w-full' />
        ))}
      </div>
    );
  }

  // The page frames the empty state: filtered-to-nothing and nothing-yet read differently.
  if (!workers.length) return <>{empty}</>;

  return (
    <Table>
      <THead>
        <Tr>
          <Th>Worker</Th>
          <Th>State</Th>
          <Th>CPU</Th>
          <Th>Memory</Th>
          <Th>Labels</Th>
          <Th>Last seen</Th>
          <Th>Version</Th>
          <Th className='text-right'>Actions</Th>
        </Tr>
      </THead>
      <TBody>
        {workers.map((w) => {
          const seen = toEpochMs(w.last_seen);
          const cpu = metric(w, 'cpu_load');
          const memMb = metric(w, 'mem_usage_mb');
          return (
            <RowLink key={w.id} to={`/workers/${w.id}`}>
              <Td>
                <Link to={`/workers/${w.id}`} className='font-mono font-medium hover:underline'>
                  {w.name}
                </Link>
                {w.description && <p className='row-subtext max-w-[40ch]'>{w.description}</p>}
              </Td>
              <Td>
                <StatusText state={workerState(w.status)}>{w.status}</StatusText>
              </Td>
              <Td>
                {cpu == null ? (
                  <span className='text-fg-subtle'>—</span>
                ) : (
                  // cpu_load is a 0..1 ratio of the worker's capacity.
                  <Meter value={cpu} label={`CPU ${Math.round(cpu * 100)}%`} />
                )}
              </Td>
              <Td className='text-fg-muted tabular-nums'>
                {memMb == null ? '—' : `${Math.round(memMb)} MB`}
              </Td>
              <Td>
                <LabelChips labels={w.labels} emptyText='none' />
              </Td>
              <Td className='text-fg-muted tabular-nums'>{seen ? relativeTime(seen) : '—'}</Td>
              <Td className='font-mono text-fg-subtle'>{w.agent_version || '—'}</Td>
              <Td className='text-right'>
                <span className='inline-flex items-center gap-1.5'>
                  {w.status === 'dead' && onReEnable && (
                    <Button
                      variant='outline'
                      size='xs'
                      disabled={reEnablingId === w.id}
                      onClick={() => onReEnable(w)}
                    >
                      <Power className='h-3 w-3' />
                      {reEnablingId === w.id ? 'Re-enabling…' : 'Re-enable'}
                    </Button>
                  )}
                  {onDelete && (
                    <IconButton
                      label={`Delete ${w.name}`}
                      variant='ghost'
                      disabled={deletingId === w.id}
                      onClick={() => onDelete(w)}
                    >
                      <Trash2 className='h-3.5 w-3.5' />
                    </IconButton>
                  )}
                </span>
              </Td>
            </RowLink>
          );
        })}
      </TBody>
    </Table>
  );
}
