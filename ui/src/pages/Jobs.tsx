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
import { Link } from 'react-router-dom';
import {
  Activity,
  ChevronRight,
  FileCode2,
  FolderOpen,
  GitBranch,
  Layers,
  Play,
  Plus,
  RefreshCw,
} from 'lucide-react';
import { listJobs, syncJobs, type SyncResult } from '@/services/jobDefService';
import { Alert } from '@/components/ui/Alert';
import { Button, LinkButton } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { facetOptions } from '@/components/ui/FacetMenu';
import { FilterBar } from '@/components/ui/FilterBar';
import { NoFilterMatches } from '@/components/ui/NoFilterMatches';
import { PageHeader } from '@/components/ui/PageHeader';
import { RowLink, TBody, Table, Td, Th, THead, Tr } from '@/components/ui/Table';
import { Skeleton } from '@/components/ui/Skeleton';
import { StatusText } from '@/components/ui/Status';
import { Tape } from '@/components/ui/Tape';
import { PageScroll } from '@/components/layout/Page';
import { useFilterList, useFilterParam } from '@/hooks/useFilterParams';
import { ConfigDialog, GitStateBadge, JobActionsMenu } from '@/features/jobs/GitState';
import { duration, percent, relativeTime } from '@/lib/format';
import type { BuildStatus, JobDefinition } from '@/types/jobs';

type Health = 'all' | 'failing' | 'running' | 'drift';

/** The folder a definition lives in — `jobdefs/release/api.yaml` → `release`. */
function folderOf(job: JobDefinition): string {
  const path = job.source.path ?? '';
  const parts = path.split('/').filter(Boolean);
  parts.pop(); // the file itself
  return parts.length ? parts.join('/') : 'root';
}

function matchesHealth(job: JobDefinition, health: Health): boolean {
  switch (health) {
    case 'failing':
      return job.lastBuild?.status === 'failed';
    case 'running':
      return job.lastBuild?.status === 'running' || job.lastBuild?.status === 'queued';
    case 'drift':
      return job.gitState !== 'synced';
    default:
      return true;
  }
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

type SortKey = 'name' | 'recent' | 'slowest' | 'flakiest';

/** Rows stay grouped by folder; sort decides the order inside each group. */
const COMPARE: Record<SortKey, (a: JobDefinition, b: JobDefinition) => number> = {
  name: (a, b) => a.name.localeCompare(b.name),
  recent: (a, b) => (b.lastBuild?.startedAt ?? 0) - (a.lastBuild?.startedAt ?? 0),
  slowest: (a, b) => b.medianDurationMs - a.medianDurationMs,
  flakiest: (a, b) => a.successRate - b.successRate,
};

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
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});

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

  const groups = useMemo(() => {
    const needle = query.trim().toLowerCase();
    const filtered = all.filter(
      (job) =>
        matchesHealth(job, health) &&
        (folders.length === 0 || folders.includes(folderOf(job))) &&
        (queues.length === 0 || queues.includes(job.queue)) &&
        (!needle ||
          job.name.toLowerCase().includes(needle) ||
          job.slug.toLowerCase().includes(needle) ||
          job.source.path.toLowerCase().includes(needle)),
    );
    const byFolder = new Map<string, JobDefinition[]>();
    for (const job of filtered) {
      const folder = folderOf(job);
      const bucket = byFolder.get(folder);
      if (bucket) bucket.push(job);
      else byFolder.set(folder, [job]);
    }
    return [...byFolder.entries()]
      .map(([folder, rows]) => ({
        folder,
        rows: [...rows].sort(COMPARE[sort] ?? COMPARE.name),
      }))
      .sort((a, b) => a.folder.localeCompare(b.folder));
  }, [all, folders, health, query, queues, sort]);

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
              description='Definitions are synced from Git. Add a YAML file under the job-definitions directory and run Sync from Git in Settings — or author one here with New template.'
            />
          </div>
        ) : shown === 0 ? (
          <NoFilterMatches noun='jobs' onReset={resetFilters} />
        ) : (
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
                          className={`h-3.5 w-3.5 transition-transform ${
                            isCollapsed ? '' : 'rotate-90'
                          }`}
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
        )}
      </PageScroll>
    </>
  );
}
