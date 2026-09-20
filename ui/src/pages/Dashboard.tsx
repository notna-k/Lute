/**
 * The overview: what is running, what is broken, what the fleet looks like.
 *
 * Built from the same three queries the other pages use, so it costs nothing
 * extra and can never disagree with them. Ordered by urgency — in-flight builds
 * first, then failures, then the fleet — because that is the order an operator
 * reads a panel in.
 */
import { useMemo, useState } from 'react';
import { Link } from 'react-router-dom';
import { ArrowRight, Plus, Server } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { useAuth } from '@/contexts/AuthContext';
import { useUserWorkers } from '@/hooks/useWorkers';
import type { Worker } from '@/types';
import { listJobs } from '@/services/jobDefService';
import { executionService } from '@/services/executionService';
import {
  Alert,
  Button,
  Card,
  CardHeader,
  CardTitle,
  EmptyState,
  LinkButton,
  PageHeader,
  RowLink,
  Skeleton,
  Slots,
  StatusText,
  TBody,
  Table,
  Tape,
  Td,
  Th,
  THead,
  Tr,
} from '@/components/ui';
import { PageBody, PageScroll, Section } from '@/components/layout';
import { AddWorkerDialog } from '@/features/workers/AddWorkerDialog';
import { workerState } from '@/features/workers/utils';
import { duration, relativeTime, toEpochMs } from '@/lib/format';

/** One headline number. Deliberately flat: the figure is the emphasis. */
function Stat({
  label,
  value,
  hint,
  tone,
  loading,
}: {
  label: string;
  value: React.ReactNode;
  hint?: React.ReactNode;
  tone?: 'danger' | 'warning';
  loading?: boolean;
}) {
  return (
    <div className='border border-border bg-surface px-4 py-3.5'>
      <p className='caption'>{label}</p>
      {loading ? (
        <Skeleton className='mt-1.5 h-7 w-14' />
      ) : (
        <p
          className={`mt-1 font-mono text-[26px] font-semibold leading-none tabular-nums ${
            tone === 'danger' ? 'text-danger' : tone === 'warning' ? 'text-warning' : 'text-fg'
          }`}
        >
          {value}
        </p>
      )}
      {hint && <p className='mt-1.5 text-[11.5px] text-fg-subtle'>{hint}</p>}
    </div>
  );
}

/** Stable empty list, so the fleet memo does not re-run on every render. */
const NO_WORKERS: Worker[] = [];

export default function Dashboard() {
  const { user } = useAuth();
  const [addOpen, setAddOpen] = useState(false);

  const workersQuery = useUserWorkers();
  const jobsQuery = useQuery({ queryKey: ['jobs'], queryFn: listJobs });
  const recentQuery = useQuery({
    queryKey: ['executions', 'recent'],
    queryFn: () => executionService.list({ limit: 8, sort: 'finished_at_desc' }),
    refetchInterval: 15000,
  });

  const workers = workersQuery.data ?? NO_WORKERS;
  const jobs = jobsQuery.data ?? [];
  const recent = recentQuery.data?.executions ?? [];

  const fleet = useMemo(() => {
    const online = workers.filter((w) => workerState(w.status) !== 'offline');
    return { total: workers.length, online: online.length };
  }, [workers]);

  const inFlight = jobs.filter(
    (j) => j.lastBuild?.status === 'running' || j.lastBuild?.status === 'queued',
  );
  const failing = jobs.filter((j) => j.lastBuild?.status === 'failed');

  return (
    <>
      <PageHeader
        title={`Welcome back, ${user?.display_name || user?.email?.split('@')[0] || 'there'}`}
        description='What the fleet is doing right now.'
        actions={
          <>
            <Button variant='secondary' size='sm' onClick={() => setAddOpen(true)}>
              <Plus className='h-3.5 w-3.5' /> Add worker
            </Button>
            <LinkButton to='/jobs' variant='primary' size='sm'>
              Run a job
            </LinkButton>
          </>
        }
      />

      <PageScroll>
        <PageBody>
          {workersQuery.isError && (
            <Alert tone='danger' className='mb-6'>
              Failed to load the worker fleet. Refresh to try again.
            </Alert>
          )}

          <div className='grid gap-3 sm:grid-cols-2 xl:grid-cols-4'>
            <Stat
              label='In flight'
              value={inFlight.length}
              tone={inFlight.length ? 'warning' : undefined}
              hint={
                inFlight.length
                  ? inFlight
                      .map((j) => j.name)
                      .slice(0, 2)
                      .join(', ')
                  : 'nothing queued or running'
              }
              loading={jobsQuery.isLoading}
            />
            <Stat
              label='Failing jobs'
              value={failing.length}
              tone={failing.length ? 'danger' : undefined}
              hint='last build did not pass'
              loading={jobsQuery.isLoading}
            />
            <Stat
              label='Definitions'
              value={jobs.length}
              hint='synced from Git'
              loading={jobsQuery.isLoading}
            />
            <Stat
              label='Workers online'
              value={`${fleet.online}/${fleet.total}`}
              tone={fleet.total && !fleet.online ? 'danger' : undefined}
              hint={<Slots total={fleet.total} used={fleet.online} />}
              loading={workersQuery.isLoading}
            />
          </div>

          <Section
            title='Needs attention'
            aside={
              <Link to='/jobs' className='inline-flex items-center gap-1 hover:text-fg'>
                All jobs <ArrowRight className='h-3 w-3' />
              </Link>
            }
          >
            <Card>
              {failing.length === 0 && inFlight.length === 0 ? (
                <div className='py-10'>
                  <EmptyState
                    title='Everything is green'
                    description='No job’s last build failed, and nothing is in flight.'
                  />
                </div>
              ) : (
                <Table className='[&_td:first-child]:pl-4 [&_th:first-child]:pl-4 [&_td:last-child]:pr-4 [&_th:last-child]:pr-4'>
                  <THead>
                    <Tr>
                      <Th>Job</Th>
                      <Th>Last build</Th>
                      <Th>When</Th>
                      <Th>History</Th>
                    </Tr>
                  </THead>
                  <TBody>
                    {[...inFlight, ...failing].map((job) => (
                      <RowLink key={job.slug} to={`/jobs/${job.slug}`}>
                        <Td>
                          <Link
                            to={`/jobs/${job.slug}`}
                            className='font-mono font-medium hover:underline'
                          >
                            {job.name}
                          </Link>
                        </Td>
                        <Td>
                          {job.lastBuild && (
                            <StatusText state={job.lastBuild.status}>
                              <span className='font-mono'>#{job.lastBuild.id}</span>
                            </StatusText>
                          )}
                        </Td>
                        <Td className='text-fg-muted tabular-nums'>
                          {job.lastBuild ? relativeTime(job.lastBuild.startedAt) : '—'}
                        </Td>
                        <Td>{job.recent?.length ? <Tape states={job.recent} /> : '—'}</Td>
                      </RowLink>
                    ))}
                  </TBody>
                </Table>
              )}
            </Card>
          </Section>

          <Section
            title='Latest runs'
            aside={
              <Link to='/executions' className='inline-flex items-center gap-1 hover:text-fg'>
                All builds <ArrowRight className='h-3 w-3' />
              </Link>
            }
          >
            <Card>
              <CardHeader>
                <CardTitle>Across every job</CardTitle>
              </CardHeader>
              {recentQuery.isLoading ? (
                <div className='space-y-2 p-4'>
                  {Array.from({ length: 5 }).map((_, i) => (
                    <Skeleton key={i} className='h-8 w-full' />
                  ))}
                </div>
              ) : recent.length === 0 ? (
                <div className='py-10'>
                  <EmptyState
                    icon={<Server className='h-5 w-5' />}
                    title='Nothing has run yet'
                    description='Trigger a job and its run will show up here.'
                  />
                </div>
              ) : (
                <Table className='[&_td:first-child]:pl-4 [&_th:first-child]:pl-4 [&_td:last-child]:pr-4 [&_th:last-child]:pr-4'>
                  <THead>
                    <Tr>
                      <Th>Status</Th>
                      <Th>Type</Th>
                      <Th>Queue</Th>
                      <Th>Finished</Th>
                      <Th className='text-right'>Duration</Th>
                    </Tr>
                  </THead>
                  <TBody>
                    {recent.map((ex) => {
                      const finished = toEpochMs(ex.finished_at);
                      return (
                        <RowLink key={ex.id} to={`/executions/${ex.job_id}`}>
                          <Td>
                            <StatusText state={ex.success ? 'passed' : 'failed'} />
                          </Td>
                          <Td className='font-mono text-fg-muted'>{ex.type}</Td>
                          <Td className='text-fg-muted'>{ex.queue}</Td>
                          <Td className='text-fg-muted tabular-nums'>
                            {finished ? relativeTime(finished) : '—'}
                          </Td>
                          <Td className='text-right tabular-nums'>{duration(ex.elapsed_ms)}</Td>
                        </RowLink>
                      );
                    })}
                  </TBody>
                </Table>
              )}
            </Card>
          </Section>
        </PageBody>
      </PageScroll>

      <AddWorkerDialog open={addOpen} onClose={() => setAddOpen(false)} />
    </>
  );
}
