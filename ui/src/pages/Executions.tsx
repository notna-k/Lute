/**
 * Every run the engine has recorded, newest first.
 *
 * This is the cross-job view: the same rows a job's Builds tab shows, without
 * the job filter. Status is the first thing the eye needs, so it leads the row
 * and carries the shape vocabulary rather than a colour alone.
 */
import { useCallback, useEffect, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { Plus, RefreshCw, X } from 'lucide-react';
import {
  Alert,
  Button,
  EmptyState,
  FilterSelect,
  IconButton,
  NativeSelect,
  Pagination,
  RowLink,
  SegmentedControl,
  Skeleton,
  StatusText,
  TBody,
  Table,
  Td,
  Th,
  THead,
  PageHeader,
  Toolbar,
  Tr,
} from '@/components/ui';
import { PageScroll } from '@/components/layout';
import { EnqueueJobDialog } from '@/features/jobs/EnqueueJobDialog';
import { executionService, type JobExecution } from '@/services/executionService';
import { duration, relativeTime, timestamp, toEpochMs } from '@/lib/format';
import { cn } from '@/lib/cn';

const PAGE_SIZE = 25;

type StatusFilter = '' | 'success' | 'failed';
type SortOption = 'finished_at_desc' | 'finished_at_asc';

/** Shortens an opaque id to something a row can hold without wrapping. */
const shortId = (id: string, keep = 10) =>
  id.length > keep + 2 ? `${id.slice(0, keep)}…` : id;

export default function Executions() {
  const navigate = useNavigate();
  const [rows, setRows] = useState<JobExecution[]>([]);
  const [total, setTotal] = useState(0);
  const [page, setPage] = useState(0);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [queueFilter, setQueueFilter] = useState('');
  const [typeFilter, setTypeFilter] = useState('');
  const [statusFilter, setStatusFilter] = useState<StatusFilter>('');
  const [sort, setSort] = useState<SortOption>('finished_at_desc');

  // null once the lookup fails: the filters then fall back to free text.
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

  const fetchExecutions = useCallback(async () => {
    setLoading(true);
    setError(null);
    try {
      const res = await executionService.list({
        queue: queueFilter.trim() || undefined,
        type: typeFilter.trim() || undefined,
        status: statusFilter || undefined,
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
  }, [queueFilter, typeFilter, statusFilter, sort, page]);

  useEffect(() => {
    void fetchExecutions();
  }, [fetchExecutions]);

  const filtered = Boolean(queueFilter || typeFilter || statusFilter);
  const clearFilters = () => {
    setQueueFilter('');
    setTypeFilter('');
    setStatusFilter('');
    setPage(0);
  };

  return (
    <>
      <PageHeader
        title='Builds'
        description='Every run the engine has recorded, across all jobs.'
        facts={<span className='tabular-nums'>{total} recorded</span>}
        actions={
          <Button variant='primary' size='sm' onClick={() => setDialogOpen(true)}>
            <Plus className='h-3.5 w-3.5' /> Trigger job
          </Button>
        }
      />

      <Toolbar>
        <SegmentedControl<StatusFilter>
          label='Filter by status'
          value={statusFilter}
          onChange={(v) => {
            setStatusFilter(v);
            setPage(0);
          }}
          options={[
            { value: '', label: 'All' },
            { value: 'success', label: 'Passed' },
            { value: 'failed', label: 'Failed' },
          ]}
        />
        <FilterSelect
          label='queue'
          allLabel='All queues'
          value={queueFilter}
          options={queueOptions}
          onChange={(v) => {
            setQueueFilter(v);
            setPage(0);
          }}
        />
        <FilterSelect
          label='type'
          allLabel='All types'
          value={typeFilter}
          options={typeOptions}
          onChange={(v) => {
            setTypeFilter(v);
            setPage(0);
          }}
        />
        <NativeSelect
          value={sort}
          aria-label='Sort order'
          className='w-[150px]'
          onChange={(e) => {
            setSort(e.target.value as SortOption);
            setPage(0);
          }}
        >
          <option value='finished_at_desc'>Newest first</option>
          <option value='finished_at_asc'>Oldest first</option>
        </NativeSelect>
        {filtered && (
          <Button variant='ghost' size='sm' onClick={clearFilters}>
            <X className='h-3.5 w-3.5' /> Clear
          </Button>
        )}
        <IconButton
          label='Refresh'
          variant='outline'
          className='ml-auto'
          onClick={() => void fetchExecutions()}
          disabled={loading}
        >
          <RefreshCw className={cn('h-3.5 w-3.5', loading && 'animate-spin')} />
        </IconButton>
      </Toolbar>

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
          <div className='py-16'>
            <EmptyState
              title={filtered ? 'No runs match these filters' : 'No runs recorded yet'}
              description={
                filtered
                  ? 'Loosen the filters, or trigger a job to produce one.'
                  : 'Trigger a job and its run will land here.'
              }
              action={
                filtered ? (
                  <Button size='sm' onClick={clearFilters}>
                    <X className='h-3.5 w-3.5' /> Clear filters
                  </Button>
                ) : (
                  <Button size='sm' onClick={() => setDialogOpen(true)}>
                    <Plus className='h-3.5 w-3.5' /> Trigger job
                  </Button>
                )
              }
            />
          </div>
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
                          <Link
                            to={`/workers/${ex.worker_id}`}
                            className='hover:underline'
                          >
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
                      <Td className='text-right tabular-nums'>
                        {duration(ex.elapsed_ms)}
                      </Td>
                      <Td
                        className={cn(
                          'max-w-[220px] truncate',
                          ex.error ? 'text-danger' : 'text-fg-subtle'
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
            <Pagination
              total={total}
              page={page}
              pageSize={PAGE_SIZE}
              onPageChange={setPage}
            />
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
