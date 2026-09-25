// Every run the engine has recorded. Filtering is server-side because the list is paginated.
import { useEffect, useState } from 'react';
import { keepPreviousData, useQuery } from '@tanstack/react-query';
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
import { executionService, type ExecutionSort } from '@/services/executionService';
import { duration, relativeTime, timestamp, toEpochMs } from '@/lib/format';
import { cn } from '@/lib/cn';

const PAGE_SIZE = 25;

type StatusFilter = 'all' | 'success' | 'failed';

const shortId = (id: string, keep = 10) => (id.length > keep + 2 ? `${id.slice(0, keep)}…` : id);

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
  const [page, setPage] = useState(0);
  const [search, setSearch] = useFilterParam<string>('q', '');
  const [queues, setQueues] = useFilterList('queue');
  const [types, setTypes] = useFilterList('type');
  const [status, setStatus] = useFilterParam<StatusFilter>('state', 'all');
  const [sort, setSort] = useFilterParam<ExecutionSort>('sort', 'finished_at_desc');
  const debouncedSearch = useDebounced(search);
  const [dialogOpen, setDialogOpen] = useState(false);

  const filterOptions = useQuery({
    queryKey: ['execution-filter-options'],
    queryFn: executionService.filterOptions,
  });
  // null once the lookup fails: the facets then fall back to free text.
  const queueOptions = filterOptions.isError ? null : (filterOptions.data?.queues ?? []);
  const typeOptions = filterOptions.isError ? null : (filterOptions.data?.types ?? []);

  // Page 4 of the old result set is not page 4 of the new one.
  useEffect(() => {
    setPage(0);
  }, [debouncedSearch, queues, types, status, sort]);

  const list = useQuery({
    queryKey: ['executions', { queues, types, status, debouncedSearch, page, sort }],
    queryFn: () =>
      executionService.list({
        queues,
        types,
        status: status === 'all' ? undefined : status,
        search: debouncedSearch,
        offset: page * PAGE_SIZE,
        limit: PAGE_SIZE,
        sort,
      }),
    placeholderData: keepPreviousData,
  });
  const rows = list.data?.executions ?? [];
  const total = list.data?.total ?? 0;
  const loading = list.isFetching;
  const error = list.error?.message;

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
            onClick={() => void list.refetch()}
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
