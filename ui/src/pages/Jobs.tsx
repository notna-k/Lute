/**
 * The job list — the panel's home for "what can I run, and is it healthy".
 *
 * Rows are grouped by the folder their definition lives in, because that is how
 * the Git repo is organised and how operators already talk about jobs ("the
 * release ones"). Each row answers three questions without a click: what did it
 * last do, has it been failing (the history strip), and how long does it take.
 */
import { useMemo, useState } from 'react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Activity, FileCode2, FolderOpen, GitBranch, Layers, Plus, RefreshCw } from 'lucide-react';
import { listJobs, syncJobs, type SyncResult } from '@/services/jobDefService';
import { Alert } from '@/components/ui/Alert';
import { Button, LinkButton } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { facetOptions } from '@/components/ui/FacetMenu';
import { FilterBar } from '@/components/ui/FilterBar';
import { NoFilterMatches } from '@/components/ui/NoFilterMatches';
import { PageHeader } from '@/components/ui/PageHeader';
import { Skeleton } from '@/components/ui/Skeleton';
import { PageScroll } from '@/components/layout/Page';
import { useFilterList, useFilterParam } from '@/hooks/useFilterParams';
import { ConfigDialog } from '@/features/jobs/GitState';
import { JobTable } from '@/features/jobs/JobTable';
import {
  folderOf,
  groupJobs,
  matchesHealth,
  type Health,
  type SortKey,
} from '@/features/jobs/jobFilters';
import type { JobDefinition } from '@/types/jobs';

/** One line of plain English for what a sync actually did. */
function syncSummary(r: SyncResult): string {
  const parts = [
    r.added && `${r.added} added`,
    r.updated && `${r.updated} updated`,
    r.detached && `${r.detached} no longer in Git`,
    r.pruned && `${r.pruned} pruned`,
  ].filter(Boolean);
  return parts.length ? `Synced — ${parts.join(', ')}.` : 'Synced — already up to date.';
}

/** Stable empty list, so the memos below do not re-run on every render. */
const NO_JOBS: JobDefinition[] = [];

export default function Jobs() {
  const queryClient = useQueryClient();
  const { data: jobs, isLoading } = useQuery({ queryKey: ['jobs'], queryFn: listJobs });
  const [exporting, setExporting] = useState(false);
  const [query, setQuery] = useFilterParam<string>('q', '');
  const [health, setHealth] = useFilterParam<Health>('state', 'all');
  const [folders, setFolders] = useFilterList('folder');
  const [queues, setQueues] = useFilterList('queue');
  const [sort, setSort] = useFilterParam<SortKey>('sort', 'name');

  const all = jobs ?? NO_JOBS;

  const sync = useMutation({
    mutationFn: syncJobs,
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      void queryClient.invalidateQueries({ queryKey: ['job'] });
    },
  });

  const counts = useMemo(
    () => ({
      all: all.length,
      failing: all.filter((j) => matchesHealth(j, 'failing')).length,
      running: all.filter((j) => matchesHealth(j, 'running')).length,
      drift: all.filter((j) => matchesHealth(j, 'drift')).length,
    }),
    [all],
  );

  // The facet menus offer what the fleet of definitions actually holds, with
  // the row count beside each value — a folder with one job is worth knowing
  // about before picking it, not after.
  const folderOptions = useMemo(() => facetOptions(all.map(folderOf)), [all]);
  const queueOptions = useMemo(() => facetOptions(all.map((j) => j.queue)), [all]);

  const groups = useMemo(
    () => groupJobs(all, { query, health, folders, queues, sort }),
    [all, folders, health, query, queues, sort],
  );

  const shown = groups.reduce((n, g) => n + g.rows.length, 0);

  function resetFilters() {
    setQuery('');
    setHealth('all');
    setFolders([]);
    setQueues([]);
  }

  return (
    <>
      <PageHeader
        title='Jobs'
        description='Definitions synced from Git. Pick one to run it or read its last builds.'
        facts={
          <>
            <span className='tabular-nums'>{counts.all} definitions</span>
            {counts.failing > 0 && (
              <span className='text-danger tabular-nums'>{counts.failing} failing</span>
            )}
            {counts.running > 0 && (
              <span className='text-warning tabular-nums'>{counts.running} in flight</span>
            )}
            {counts.drift > 0 && (
              <span className='text-warning tabular-nums'>{counts.drift} differ from Git</span>
            )}
          </>
        }
        actions={
          <>
            <Button
              variant='secondary'
              size='sm'
              loading={sync.isPending}
              onClick={() => sync.mutate()}
            >
              <RefreshCw className='h-3.5 w-3.5' /> Sync from Git
            </Button>
            <Button variant='secondary' size='sm' onClick={() => setExporting(true)}>
              <FileCode2 className='h-3.5 w-3.5' /> Export config
            </Button>
            <LinkButton to='/jobs/new' variant='secondary' size='sm'>
              <Plus className='h-3.5 w-3.5' /> New template
            </LinkButton>
          </>
        }
      />

      <ConfigDialog open={exporting} onClose={() => setExporting(false)} />

      <FilterBar<Health>
        search={{
          value: query,
          onChange: setQuery,
          placeholder: 'Search by name, slug or path',
          label: 'Search jobs',
          chipLabel: 'name',
        }}
        scope={{
          label: 'health',
          allLabel: 'Any health',
          allCount: counts.all,
          icon: <Activity className='h-3.5 w-3.5 text-fg-subtle' />,
          value: health,
          onChange: setHealth,
          options: [
            { value: 'all', label: 'All', count: counts.all },
            { value: 'failing', label: 'Failing', count: counts.failing },
            { value: 'running', label: 'In flight', count: counts.running },
            {
              value: 'drift',
              label: 'Differs from Git',
              count: counts.drift,
              title: 'Definitions the panel changed, created or lost',
            },
          ],
        }}
        facets={[
          {
            id: 'folder',
            label: 'folder',
            allLabel: 'All folders',
            icon: <FolderOpen className='h-3.5 w-3.5 text-fg-subtle' />,
            values: folders,
            options: folderOptions,
            onChange: setFolders,
          },
          {
            id: 'queue',
            label: 'queue',
            allLabel: 'All queues',
            icon: <Layers className='h-3.5 w-3.5 text-fg-subtle' />,
            values: queues,
            options: queueOptions,
            onChange: setQueues,
          },
        ]}
        sort={{
          value: sort,
          onChange: (v) => setSort(v as SortKey),
          options: [
            { value: 'name', label: 'Name A–Z' },
            { value: 'recent', label: 'Last run first' },
            { value: 'slowest', label: 'Slowest first' },
            { value: 'flakiest', label: 'Least reliable' },
          ],
        }}
        count={{ shown, total: counts.all, noun: 'jobs' }}
        onReset={resetFilters}
      />

      <PageScroll>
        {(sync.isSuccess || sync.isError) && (
          <div className='px-7 pt-4'>
            <Alert tone={sync.isError ? 'danger' : 'success'}>
              {sync.isError ? (sync.error as Error).message : syncSummary(sync.data)}
              {sync.data?.skipped.map((s) => (
                <span key={s} className='mt-1 block font-mono text-xs text-warning'>
                  skipped {s}
                </span>
              ))}
            </Alert>
          </div>
        )}
        {isLoading ? (
          <div className='space-y-2 px-7 py-6'>
            {Array.from({ length: 6 }).map((_, i) => (
              <Skeleton key={i} className='h-9 w-full' />
            ))}
          </div>
        ) : all.length === 0 ? (
          <div className='py-16'>
            <EmptyState
              icon={<GitBranch className='h-5 w-5' />}
              title='No job definitions'
              description='Definitions are synced from Git. Add a YAML file under the job-definitions directory and run Sync from Git — or author one here with New template.'
            />
          </div>
        ) : shown === 0 ? (
          <NoFilterMatches noun='jobs' onReset={resetFilters} />
        ) : (
          <JobTable groups={groups} />
        )}
      </PageScroll>
    </>
  );
}
