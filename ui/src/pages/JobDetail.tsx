/**
 * One job: its builds, a form to run it, and the definition behind it.
 *
 * The three views are routes rather than local tab state, so a build, a
 * half-filled run form or the YAML can all be linked to and reloaded. The header
 * is fixed; only the view below it scrolls, which is what lets a log stream
 * without the page drifting.
 */
import { useMemo } from 'react';
import { useLocation, useNavigate, useParams } from 'react-router-dom';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { GitBranch, Play, Save } from 'lucide-react';
import { getJob, listBuilds, triggerBuild, updateJob } from '@/services/jobDefService';
import { ApiError } from '@/services/api';
import { BuildWorkbench } from '@/features/jobs/BuildWorkbench';
import { BuildList } from '@/features/jobs/BuildList';
import { BuildPane } from '@/features/jobs/BuildPane';
import {
  Alert,
  Button,
  Fact,
  LinkTabs,
  Spinner,
  Tape,
  toastSubject,
  useToast,
} from '@/components/ui';
import { DetailHeader, PageBody, PageScroll } from '@/components/layout';
import { GitStateBadge, JobActionsMenu } from '@/features/jobs/GitState';
import { duration, percent } from '@/lib/format';
import type { Build, ParameterField, ParameterValues } from '@/types/jobs';

type View = 'builds' | 'run' | 'config';

/** Stable empty list, so the selected-build memo has stable dependencies. */
const NO_BUILDS: Build[] = [];

/** Which of the three views the current URL selects. */
function viewOf(pathname: string, slug: string): View {
  const rest = pathname.replace(`/jobs/${slug}`, '');
  if (rest.startsWith('/run')) return 'run';
  if (rest.startsWith('/config')) return 'config';
  return 'builds';
}

export default function JobDetail() {
  const { slug = '', buildId } = useParams();
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { toast } = useToast();
  const view = viewOf(useLocation().pathname, slug);

  const { data: job, isLoading } = useQuery({
    queryKey: ['job', slug],
    queryFn: () => getJob(slug),
  });

  const { data: builds } = useQuery({
    queryKey: ['builds', slug],
    queryFn: () => listBuilds(slug),
    // Builds move through queued → running → passed/failed on the worker, so
    // keep polling while any of them is still in flight.
    refetchInterval: (query) =>
      query.state.data?.some((b) => b.status === 'queued' || b.status === 'running')
        ? 2000
        : 15000,
  });

  const buildRows = builds ?? NO_BUILDS;
  const selected = useMemo(
    () => buildRows.find((b) => b.id === buildId) ?? buildRows[0],
    [buildRows, buildId]
  );

  const trigger = useMutation({
    // The authored schema goes with the values: the server validates against
    // what the user actually saw, so an added parameter is applied rather than
    // silently dropped.
    mutationFn: ({
      values,
      fields,
    }: {
      values: ParameterValues;
      fields: ParameterField[];
    }) => triggerBuild(slug, values, fields),
    onSuccess: (build) => {
      void queryClient.invalidateQueries({ queryKey: ['builds', slug] });
      void queryClient.invalidateQueries({ queryKey: ['job', slug] });
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      toast({
        message: (
          <>
            Queued <span className={toastSubject}>#{build.id}</span>
          </>
        ),
        link: { to: `/jobs/${slug}/builds/${build.id}`, label: 'Watch' },
      });
      navigate(`/jobs/${slug}/builds/${build.id}`);
    },
  });

  const saveEdit = useMutation({
    mutationFn: (parameters: ParameterField[]) =>
      updateJob(slug, {
        name: job?.name ?? '',
        description: job?.description ?? '',
        queue: job?.queue ?? 'default',
        runtime: job?.runtime ?? '',
        command: job?.command ?? '',
        sourceRepo: job?.source.repo || undefined,
        labelSelector: job?.labelSelector,
        parameters,
      }),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['job', slug] });
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      toast({ message: 'Definition saved' });
    },
  });

  if (isLoading) {
    return (
      <div className='flex h-full items-center justify-center'>
        <Spinner size={28} />
      </div>
    );
  }

  if (!job) {
    return (
      <PageScroll>
        <PageBody>
          <Alert tone='danger' title='Job not found'>
            No definition is registered under <code>{slug}</code>. It may have been
            pruned by a Git sync.
          </Alert>
        </PageBody>
      </PageScroll>
    );
  }

  const fieldErrors = trigger.error instanceof ApiError ? trigger.error.fields : undefined;
  // A field-level rejection is already rendered on the inputs; repeating the
  // summary line above them would just say "invalid parameters" twice.
  const runError =
    trigger.isError && !fieldErrors ? (trigger.error as Error).message : undefined;

  // Any definition can be saved. One that came from Git then differs from it
  // until its file changes — or until the saved config is committed.
  const editFooter = (parameters: ParameterField[]) => (
    <div className='flex flex-wrap items-center gap-3 border-t border-border pt-4'>
      <Button
        variant='primary'
        disabled={saveEdit.isPending}
        onClick={() => saveEdit.mutate(parameters)}
      >
        <Save className='h-3.5 w-3.5' />
        {saveEdit.isPending ? 'Saving…' : 'Save changes'}
      </Button>
      {saveEdit.isSuccess && !saveEdit.isPending && (
        <span className='text-xs text-fg-muted'>
          Saved.{' '}
          {job.source.path && 'Commit the config to keep it past the next change in Git.'}
        </span>
      )}
      {saveEdit.isError && (
        <span className='text-xs text-danger'>{(saveEdit.error as Error).message}</span>
      )}
    </div>
  );

  const hasStats = job.medianDurationMs > 0 || job.successRate > 0;

  return (
    <>
      <DetailHeader
        crumbs={[{ label: 'Jobs', to: '/jobs' }]}
        title={job.name}
        subtitle={job.description}
        tags={<GitStateBadge state={job.gitState} />}
        actions={
          <>
            <Button variant='primary' size='sm' onClick={() => navigate(`/jobs/${slug}/run`)}>
              <Play className='h-3.5 w-3.5' /> Run build
            </Button>
            <JobActionsMenu job={job} />
          </>
        }
        tabs={
          <LinkTabs
            items={[
              {
                to: `/jobs/${slug}`,
                label: 'Builds',
                count: buildRows.length,
                active: view === 'builds',
              },
              { to: `/jobs/${slug}/run`, label: 'Run', active: view === 'run' },
              {
                to: `/jobs/${slug}/config`,
                label: 'Definition',
                active: view === 'config',
              },
            ]}
          />
        }
        meta={
          <>
            {job.recent?.length ? <Tape states={job.recent} /> : null}
            {hasStats && (
              <Fact title='Success rate over the trailing 30 days'>
                <span className='tabular-nums'>{percent(job.successRate)} · 30d</span>
              </Fact>
            )}
            {job.medianDurationMs > 0 && (
              <Fact title='Median build duration'>
                <span className='tabular-nums'>~{duration(job.medianDurationMs)}</span>
              </Fact>
            )}
            <Fact title='Queue and runtime'>
              <span className='font-mono'>
                {job.queue} · {job.runtime}
              </span>
            </Fact>
            <Fact icon={<GitBranch className='h-3 w-3' />} title={job.source.repo}>
              {job.source.path ? (
                <span
                  className={
                    job.gitState === 'removed' ? 'font-mono line-through' : 'font-mono'
                  }
                >
                  {job.source.path}
                  {job.source.commit ? `@${job.source.commit}` : ''}
                </span>
              ) : (
                <span className='font-mono'>not in Git</span>
              )}
            </Fact>
          </>
        }
      />

      {view === 'builds' ? (
        <div className='flex min-h-0 flex-1 max-md:flex-col'>
          <BuildList
            builds={buildRows}
            selectedId={selected?.id}
            linkTo={(build) => `/jobs/${slug}/builds/${build.id}`}
            className='w-[248px] shrink-0 max-md:w-full'
          />
          <BuildPane
            job={job}
            build={selected}
            onRerun={() => navigate(`/jobs/${slug}/run`)}
          />
        </div>
      ) : (
        <PageScroll>
          <PageBody>
            <BuildWorkbench
              job={job}
              builds={buildRows}
              mode={view === 'run' ? 'run' : 'edit'}
              onRun={(values, fields) => trigger.mutate({ values, fields })}
              running={trigger.isPending}
              serverErrors={fieldErrors}
              runError={runError}
              footer={view === 'config' ? editFooter : undefined}
            />
          </PageBody>
        </PageScroll>
      )}
    </>
  );
}
