/**
 * Every run the engine has recorded, newest first.
 *
 * This is the cross-job view: the same rows a job's Builds tab shows, without
 * the job filter. Status is the first thing the eye needs, so it leads the row
 * and carries the shape vocabulary rather than a colour alone.
 *
 * Filtering happens on the server — the list is paginated, so a client-side
 * search would only ever search the twenty-five rows already on screen and
 * quietly lie about the rest.
 */
import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Boxes, CircleDot, Layers, Plus, RefreshCw } from 'lucide-react';
import { Alert } from '@/components/ui/Alert';
import { Button } from '@/components/ui/Button';
import { EmptyState } from '@/components/ui/EmptyState';
import { facetOptions } from '@/components/ui/FacetMenu';
import { FilterBar } from '@/components/ui/FilterBar';
import { NoFilterMatches } from '@/components/ui/NoFilterMatches';
import { IconButton } from '@/components/ui/IconButton';
import { Pagination } from '@/components/ui/Pagination';
import { RowLink, TBody, Table, Td, Th, THead, Tr } from '@/components/ui/Table';
import { Skeleton } from '@/components/ui/Skeleton';
import { StatusText } from '@/components/ui/Status';
import { PageHeader } from '@/components/ui/PageHeader';
import { PageScroll } from '@/components/layout/Page';
import { useFilterList, useFilterParam } from '@/hooks/useFilterParams';
import { EnqueueJobDialog } from '@/features/jobs/EnqueueJobDialog';
import {
  executionService,
  type ExecutionSort,
  type JobExecution,
} from '@/services/executionService';
import { duration, relativeTime, timestamp, toEpochMs } from '@/lib/format';
import { cn } from '@/lib/cn';

const PAGE_SIZE = 25;

type StatusFilter = 'all' | 'success' | 'failed';

/** Shortens an opaque id to something a row can hold without wrapping. */
const shortId = (id: string, keep = 10) => (id.length > keep + 2 ? `${id.slice(0, keep)}…` : id);

/** Debounces the search box: one request per pause, not one per keystroke. */
function useDebounced<T>(value: T, ms = 250): T {
  const [settled, setSettled] = useState(value);
  useEffect(() => {
    const t = setTimeout(() => setSettled(value), ms);
    return () => clearTimeout(t);
  }, [value, ms]);
  return settled;
}

export default function Executions() {
  const navigate = useNavigate();
  const [rows, setRows] = useState<JobExecution[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useFilterParam<string>('q', '');
  const [queues, setQueues] = useFilterList('queue');
  const [types, setTypes] = useFilterList('type');
  const [status, setStatus] = useFilterParam<StatusFilter>('state', 'all');
  const [sort, setSort] = useFilterParam<ExecutionSort>('sort', 'finished_at_desc');
  const debouncedSearch = useDebounced(search);

  // null once the lookup fails: the facets then fall back to free text.
  const [queueOptions, setQueueOptions] = useState<string[] | null>([]);
  const [typeOptions, setTypeOptions] = useState<string[] | null>([]);
  const [dialogOpen, setDialogOpen] = useState(false);

  useEffect(() => {
    void (async () => {
      try {
        const o = await executionService.filterOptions();
        setQueueOptions(o.queues ?? []);
        setTypeOptions(o.types ?? []);
      } catch {
        setQueueOptions(null);
        setTypeOptions(null);
      }
    })();
  }, []);

  // Any change to what is being asked for starts again at the first page: page
  // 4 of the old result set is not page 4 of the new one.
  useEffect(() => {
    setPage(0);
  }, [debouncedSearch, queues, types, status, sort]);

  const fetchExecutions = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await executionService.list({
        queues,
        types,
        status: status === 'all' ? undefined : status,
        search: debouncedSearch,
        offset: page * PAGE_SIZE,
        limit: PAGE_SIZE,
        sort,
      });
      setRows(res.executions ?? []);
      setTotal(res.total ?? 0);
    } catch (e) {
      setError(e instanceof Error ? e.message : 'Failed to load executions');
      setRows([]);
      setTotal(0);
    } finally {
      setLoading(false);
    }
  }, [debouncedSearch, page, queues, sort, status, types]);

  useEffect(() => {
    void fetchExecutions();
  }, [fetchExecutions]);

  const filtering =
    Boolean(search.trim()) || queues.length > 0 || types.length > 0 || status !== 'all';

  function resetFilters() {
    setSearch('');
    setQueues([]);
    setTypes([]);
    setStatus('all');
    setPage(0);
  }

  return (
    <>
      <PageHeader
        title='Builds'
        description='Every run the engine has recorded, across all jobs.'
        facts={
          <span className='tabular-nums'>
            {total} {filtering ? 'matching' : 'recorded'}
          </span>
        }
        actions={
          <Button variant='primary' size='sm' onClick={() => setDialogOpen(true)}>
            <Plus className='h-3.5 w-3.5' /> Trigger job
          </Button>
        }
      />

      <FilterBar<StatusFilter>
        search={{
          value: search,
          onChange: setSearch,
          placeholder: 'Search by run, worker or error',
          label: 'Search builds',
          chipLabel: 'text',
        }}
        scope={{
          label: 'result',
          allLabel: 'Any result',
          allCount: undefined,
          icon: <CircleDot className='h-3.5 w-3.5 text-fg-subtle' />,
          value: status,
          onChange: setStatus,
          options: [
            { value: 'all', label: 'All' },
            { value: 'success', label: 'Passed' },
            { value: 'failed', label: 'Failed' },
          ],
        }}
        facets={[
          {
            id: 'queue',
            label: 'queue',
            allLabel: 'All queues',
            icon: <Layers className='h-3.5 w-3.5 text-fg-subtle' />,
            values: queues,
            options: queueOptions && facetOptions(queueOptions),
            onChange: setQueues,
          },
          {
            id: 'type',
            label: 'type',
            allLabel: 'All types',
            icon: <Boxes className='h-3.5 w-3.5 text-fg-subtle' />,
            values: types,
            options: typeOptions && facetOptions(typeOptions),
            onChange: setTypes,
          },
        ]}
        sort={{
          value: sort,
          onChange: (v) => setSort(v as ExecutionSort),
          options: [
            { value: 'finished_at_desc', label: 'Newest first' },
            { value: 'finished_at_asc', label: 'Oldest first' },
            { value: 'elapsed_desc', label: 'Longest first' },
            { value: 'elapsed_asc', label: 'Shortest first' },
          ],
        }}
        count={{ total, noun: 'builds' }}
        onReset={resetFilters}
        actions={
          <IconButton
            label='Refresh'
            variant='outline'
            onClick={() => void fetchExecutions()}
            disabled={loading}
          >
            <RefreshCw className={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
          </IconButton>
        }
      />

      <PageScroll>
        {error && (
          <div className='px-7 pt-4'>
            <Alert tone='danger'>{error}</Alert>
          </div>
        )}

        {loading && rows.length === 0 ? (
          <div className='space-y-2 px-7 py-6'>
            {Array.from({ length: 8 }).map((_, i) => (
              <Skeleton key={i} className='h-9 w-full' />
            ))}
          </div>
        ) : rows.length === 0 ? (
          filtering ? (
            <NoFilterMatches noun='builds' onReset={resetFilters} />
          ) : (
            <div className='py-16'>
              <EmptyState
                title='No runs recorded yet'
                description='Trigger a job and its run will land here.'
                action={
                  <Button size='sm' onClick={() => setDialogOpen(true)}>
                    <Plus className='h-3.5 w-3.5' /> Trigger job
                  </Button>
                }
              />
            </div>
          )
        ) : (
          <>
            <Table>
              <THead>
                <Tr>
                  <Th>Status</Th>
                  <Th>Run</Th>
                  <Th>Type</Th>
                  <Th>Queue</Th>
                  <Th>Worker</Th>
                  <Th>Finished</Th>
                  <Th className='text-right'>Duration</Th>
                  <Th>Error</Th>
                </Tr>
              </THead>
              <TBody>
                {rows.map((ex) => {
                  const finished = toEpochMs(ex.finished_at);
                  return (
                    <RowLink key={ex.id} to={`/executions/${ex.job_id}`}>
                      <Td>
                        <StatusText state={ex.success ? 'passed' : 'failed'} />
                      </Td>
                      <Td>
                        <Link
                          to={`/executions/${ex.job_id}`}
                          className='font-mono font-medium hover:underline'
                          title={ex.job_id}
                        >
                          {shortId(ex.job_id)}
                        </Link>
                      </Td>
                      <Td className='font-mono text-fg-muted'>{ex.type}</Td>
                      <Td className='text-fg-muted'>{ex.queue}</Td>
                      <Td className='font-mono text-fg-muted' title={ex.worker_id}>
                        {ex.worker_id ? (
                          <Link to={`/workers/${ex.worker_id}`} className='hover:underline'>
                            {shortId(ex.worker_id, 8)}
                          </Link>
                        ) : (
                          '—'
                        )}
                      </Td>
                      <Td
                        className='text-fg-muted tabular-nums'
                        title={finished ? timestamp(finished) : undefined}
                      >
                        {finished ? relativeTime(finished) : '—'}
                      </Td>
                      <Td className='text-right tabular-nums'>{duration(ex.elapsed_ms)}</Td>
                      <Td
                        className={cn(
                          'max-w-[220px] truncate',
                          ex.error ? 'text-danger' : 'text-fg-subtle',
                        )}
                        title={ex.error || ''}
                      >
                        {ex.error || '—'}
                      </Td>
                    </RowLink>
                  );
                })}
              </TBody>
            </Table>
            <Pagination total={total} page={page} pageSize={PAGE_SIZE} onPageChange={setPage} />
          </>
        )}
      </PageScroll>

      <EnqueueJobDialog
        open={dialogOpen}
        onClose={() => setDialogOpen(false)}
        defaultQueue='default'
        onEnqueued={(jobId) => navigate(`/executions/${jobId}`)}
      />
    </>
  );
}
