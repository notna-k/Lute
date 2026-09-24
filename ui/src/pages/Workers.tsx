/**
 * The worker fleet.
 *
 * Two questions get asked here, and the bar splits them: *which machine* —
 * search, over name, description and address — and *which machines* — the
 * facets, over the labels and agent versions the fleet actually reports. Labels
 * are how jobs are routed, so "which machines would `gpu=true` land on" is
 * answered by picking a value rather than by recalling its spelling.
 */
import { useMemo, useState } from 'react';
import { Plus, Server, Signal, Tag, Tags } from 'lucide-react';
import { useDeleteWorker, useReEnableWorker, useUserWorkers } from '@/hooks/useWorkers';
import { useFilterList, useFilterParam } from '@/hooks/useFilterParams';
import type { Worker } from '@/types';
import {
  Alert,
  Button,
  EmptyState,
  facetOptions,
  FilterBar,
  NoFilterMatches,
  PageHeader,
} from '@/components/ui';
import { PageScroll } from '@/components/layout';
import { AddWorkerDialog } from '@/features/workers/AddWorkerDialog';
import { DeleteWorkerDialog } from '@/features/workers/DeleteWorkerDialog';
import { WorkerList } from '@/features/workers/WorkerList';
import { metric, workerState } from '@/features/workers/utils';
import { toEpochMs } from '@/lib/format';

type Availability = 'all' | 'online' | 'offline';
type SortKey = 'name' | 'seen' | 'load';

/** A worker's labels as the `key=value` strings the facet filters on. */
function labelPairs(w: Worker): string[] {
  return Object.entries(w.labels ?? {}).map(([k, v]) => `${k}=${v}`);
}

/** Selected labels narrow together: a worker must carry every one of them. */
function matchesLabels(w: Worker, selected: string[]): boolean {
  if (selected.length === 0) return true;
  const pairs = new Set(labelPairs(w));
  return selected.every((pair) => pairs.has(pair));
}

function matchesQuery(w: Worker, needle: string): boolean {
  if (!needle) return true;
  return [w.name, w.description, w.agent_ip, w.id]
    .filter(Boolean)
    .some((field) => (field as string).toLowerCase().includes(needle));
}

const COMPARE: Record<SortKey, (a: Worker, b: Worker) => number> = {
  name: (a, b) => a.name.localeCompare(b.name),
  seen: (a, b) => (toEpochMs(b.last_seen) ?? 0) - (toEpochMs(a.last_seen) ?? 0),
  load: (a, b) => (metric(b, 'cpu_load') ?? -1) - (metric(a, 'cpu_load') ?? -1),
};

/** Stable empty list, so the filter memo does not re-run on every render. */
const NO_WORKERS: Worker[] = [];

export default function Workers() {
  const [addOpen, setAddOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Worker | null>(null);

  const [query, setQuery] = useFilterParam<string>('q', '');
  const [availability, setAvailability] = useFilterParam<Availability>('state', 'all');
  const [labels, setLabels] = useFilterList('label');
  const [versions, setVersions] = useFilterList('version');
  const [sort, setSort] = useFilterParam<SortKey>('sort', 'name');

  const userQuery = useUserWorkers();
  const reEnable = useReEnableWorker();
  const remove = useDeleteWorker();

  const all = userQuery.data ?? NO_WORKERS;
  const online = all.filter((w) => workerState(w.status) !== 'offline').length;

  const labelOptions = useMemo(() => facetOptions(all.flatMap(labelPairs)), [all]);
  const versionOptions = useMemo(() => facetOptions(all.map((w) => w.agent_version)), [all]);

  const workers = useMemo(() => {
    const needle = query.trim().toLowerCase();
    return all
      .filter((w) => {
        if (!matchesQuery(w, needle)) return false;
        if (!matchesLabels(w, labels)) return false;
        if (versions.length && !versions.includes(w.agent_version ?? '')) return false;
        const offline = workerState(w.status) === 'offline';
        if (availability === 'online') return !offline;
        if (availability === 'offline') return offline;
        return true;
      })
      .sort(COMPARE[sort] ?? COMPARE.name);
  }, [all, availability, labels, query, sort, versions]);

  function resetFilters() {
    setQuery('');
    setAvailability('all');
    setLabels([]);
    setVersions([]);
  }

  const filtering =
    Boolean(query.trim()) || availability !== 'all' || labels.length > 0 || versions.length > 0;

  return (
    <>
      <PageHeader
        title='Workers'
        description='The machines builds are dispatched to. Labels decide what lands where.'
        facts={
          <span className='tabular-nums'>
            {online}/{all.length} online
          </span>
        }
        actions={
          <Button variant='primary' size='sm' onClick={() => setAddOpen(true)}>
            <Plus className='h-3.5 w-3.5' /> Add worker
          </Button>
        }
      />

      <FilterBar<Availability>
        search={{
          value: query,
          onChange: setQuery,
          placeholder: 'Search by name, address or id',
          label: 'Search workers',
          chipLabel: 'name',
        }}
        scope={{
          label: 'state',
          allLabel: 'Any state',
          allCount: all.length,
          icon: <Signal className='h-3.5 w-3.5 text-fg-subtle' />,
          value: availability,
          onChange: setAvailability,
          options: [
            { value: 'all', label: 'All', count: all.length },
            { value: 'online', label: 'Online', count: online },
            { value: 'offline', label: 'Offline', count: all.length - online },
          ],
        }}
        facets={[
          {
            id: 'label',
            label: 'label',
            allLabel: 'All labels',
            icon: <Tag className='h-3.5 w-3.5 text-fg-subtle' />,
            values: labels,
            options: labelOptions,
            onChange: setLabels,
            menuWidth: 280,
          },
          {
            id: 'version',
            label: 'version',
            allLabel: 'All versions',
            icon: <Tags className='h-3.5 w-3.5 text-fg-subtle' />,
            values: versions,
            options: versionOptions,
            onChange: setVersions,
          },
        ]}
        sort={{
          value: sort,
          onChange: (v) => setSort(v as SortKey),
          options: [
            { value: 'name', label: 'Name A–Z' },
            { value: 'seen', label: 'Last seen first' },
            { value: 'load', label: 'Busiest first' },
          ],
        }}
        count={{ shown: workers.length, total: all.length, noun: 'workers' }}
        onReset={resetFilters}
      />

      <PageScroll>
        {userQuery.isError && (
          <div className='px-7 pt-4'>
            <Alert tone='danger' title='Failed to load workers'>
              {userQuery.error instanceof Error ? userQuery.error.message : 'Unknown error'}
            </Alert>
          </div>
        )}

        <WorkerList
          workers={workers}
          loading={userQuery.isLoading}
          onReEnable={(w) => reEnable.mutate(w.id, { onSuccess: () => void userQuery.refetch() })}
          onDelete={(w) => setDeleteTarget(w)}
          reEnablingId={reEnable.isPending ? (reEnable.variables as string | undefined) : undefined}
          deletingId={remove.isPending ? (remove.variables as string | undefined) : undefined}
          empty={
            filtering && all.length ? (
              <NoFilterMatches noun='workers' onReset={resetFilters} />
            ) : (
              <div className='py-16'>
                <EmptyState
                  icon={<Server className='h-5 w-5' />}
                  title='No workers yet'
                  description='Register a machine to start running builds.'
                  action={
                    <Button size='sm' onClick={() => setAddOpen(true)}>
                      <Plus className='h-3.5 w-3.5' /> Add your first worker
                    </Button>
                  }
                />
              </div>
            )
          }
        />
      </PageScroll>

      <AddWorkerDialog open={addOpen} onClose={() => setAddOpen(false)} />
      <DeleteWorkerDialog
        worker={deleteTarget}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={() =>
          deleteTarget && remove.mutate(deleteTarget.id, { onSuccess: () => setDeleteTarget(null) })
        }
        pending={remove.isPending}
      />
    </>
  );
}
