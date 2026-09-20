/**
 * The worker fleet.
 *
 * Filtering is by label rather than by name, because that is how jobs are routed
 * — "which machines would `gpu=true` land on" is the question an operator
 * actually asks here.
 */
import { useMemo, useState } from 'react';
import { Plus, Server } from 'lucide-react';
import {
  useDeleteWorker,
  useReEnableWorker,
  useUserWorkers,
} from '@/hooks/useWorkers';
import type { Worker } from '@/types';
import {
  Alert,
  Button,
  EmptyState,
  PageHeader,
  SearchInput,
  SegmentedControl,
  Toolbar,
} from '@/components/ui';
import { PageScroll } from '@/components/layout';
import { AddWorkerDialog } from '@/features/workers/AddWorkerDialog';
import { DeleteWorkerDialog } from '@/features/workers/DeleteWorkerDialog';
import { WorkerList } from '@/features/workers/WorkerList';
import { workerState } from '@/features/workers/utils';

type Availability = 'all' | 'online' | 'offline';

/** `gpu=true, zone=eu` → `{gpu: 'true', zone: 'eu'}`. */
function parseLabelFilter(raw: string): Record<string, string> {
  const out: Record<string, string> = {};
  raw.split(',').forEach((part) => {
    const [k, ...rest] = part.trim().split('=');
    if (k) out[k.trim()] = rest.join('=').trim();
  });
  return out;
}

function matchesLabels(w: Worker, filter: Record<string, string>): boolean {
  return Object.entries(filter).every(
    ([k, v]) => !k || (w.labels ?? {})[k] === v
  );
}

/** Stable empty list, so the filter memo does not re-run on every render. */
const NO_WORKERS: Worker[] = [];

export default function Workers() {
  const [addOpen, setAddOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Worker | null>(null);
  const [labelFilter, setLabelFilter] = useState('');
  const [availability, setAvailability] = useState<Availability>('all');

  const userQuery = useUserWorkers();
  const reEnable = useReEnableWorker();
  const remove = useDeleteWorker();

  const all = userQuery.data ?? NO_WORKERS;
  const online = all.filter((w) => workerState(w.status) !== 'offline').length;

  const parsedFilter = useMemo(() => parseLabelFilter(labelFilter), [labelFilter]);
  const workers = useMemo(() => {
    return all.filter((w) => {
      if (labelFilter.trim() && !matchesLabels(w, parsedFilter)) return false;
      const offline = workerState(w.status) === 'offline';
      if (availability === 'online') return !offline;
      if (availability === 'offline') return offline;
      return true;
    });
  }, [all, availability, labelFilter, parsedFilter]);

  return (
    <>
      <PageHeader
        title='Workers'
        description='The machines builds are dispatched to. Labels decide what lands where.'
        facts={
          <>
            <span className='tabular-nums'>
              {online}/{all.length} online
            </span>
          </>
        }
        actions={
          <Button variant='primary' size='sm' onClick={() => setAddOpen(true)}>
            <Plus className='h-3.5 w-3.5' /> Add worker
          </Button>
        }
      />

      <Toolbar>
        <SearchInput
          value={labelFilter}
          onChange={(e) => setLabelFilter(e.target.value)}
          placeholder='Filter by label, e.g. gpu=true'
          aria-label='Filter workers by label'
        />
        <SegmentedControl<Availability>
          label='Filter by availability'
          value={availability}
          onChange={setAvailability}
          options={[
            { value: 'all', label: 'All', count: all.length },
            { value: 'online', label: 'Online', count: online },
            { value: 'offline', label: 'Offline', count: all.length - online },
          ]}
        />
        <span className='ml-auto font-mono text-[11.5px] text-fg-subtle tabular-nums'>
          {workers.length}/{all.length}
        </span>
      </Toolbar>

      <PageScroll>
        {userQuery.isError && (
          <div className='px-7 pt-4'>
            <Alert tone='danger' title='Failed to load workers'>
              {userQuery.error instanceof Error
                ? userQuery.error.message
                : 'Unknown error'}
            </Alert>
          </div>
        )}

        <WorkerList
          workers={workers}
          loading={userQuery.isLoading}
          onReEnable={(w) =>
            reEnable.mutate(w.id, { onSuccess: () => void userQuery.refetch() })
          }
          onDelete={(w) => setDeleteTarget(w)}
          reEnablingId={
            reEnable.isPending ? (reEnable.variables as string | undefined) : undefined
          }
          deletingId={
            remove.isPending ? (remove.variables as string | undefined) : undefined
          }
          empty={
            <EmptyState
              icon={<Server className='h-5 w-5' />}
              title={all.length ? 'No workers match' : 'No workers yet'}
              description={
                all.length
                  ? 'Clear the label filter to see the whole fleet.'
                  : 'Register a machine to start running builds.'
              }
              action={
                all.length ? undefined : (
                  <Button size='sm' onClick={() => setAddOpen(true)}>
                    <Plus className='h-3.5 w-3.5' /> Add your first worker
                  </Button>
                )
              }
            />
          }
        />
      </PageScroll>

      <AddWorkerDialog open={addOpen} onClose={() => setAddOpen(false)} />
      <DeleteWorkerDialog
        worker={deleteTarget}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={() =>
          deleteTarget &&
          remove.mutate(deleteTarget.id, { onSuccess: () => setDeleteTarget(null) })
        }
        pending={remove.isPending}
      />
    </>
  );
}
