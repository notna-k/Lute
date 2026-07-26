/**
 * From-scratch template authoring.
 *
 * This page only *saves* — there is nothing to run yet. The template is stored
 * with origin=panel, which keeps the Git sync from rewriting or pruning it, and
 * you run it from its detail page like any other definition.
 */
import { useMemo, useState } from 'react';
import { Link, useNavigate } from 'react-router-dom';
import { useMutation, useQueryClient } from '@tanstack/react-query';
import { ArrowLeft, Save } from 'lucide-react';

import { Alert, Button, Card, Input, PageHeader } from '@/components/ui';
import { BuildWorkbench } from '@/features/jobs/BuildWorkbench';
import { createJob } from '@/services/jobDefService';
import type { JobDefinition, ParameterField } from '@/types/jobs';

/**
 * Stable empty schema. The workbench reseeds its draft whenever the incoming
 * parameters change by content, so this must not be rebuilt each render — a
 * fresh [] every time would wipe the fields as they are authored.
 */
const NO_PARAMETERS: ParameterField[] = [];

export default function JobNew() {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const [name, setName] = useState('');
  const [description, setDescription] = useState('');
  const [queue, setQueue] = useState('default');
  const [runtime, setRuntime] = useState('');
  const [command, setCommand] = useState('');
  const [sourceRepo, setSourceRepo] = useState('');

  // A stand-in definition so the workbench can render. Empty source marks it as
  // uncommitted, which is what drives the "new" badge.
  const draftJob = useMemo<JobDefinition>(
    () => ({
      slug: '',
      name: name || 'Untitled template',
      description,
      queue,
      labelSelector: {},
      runtime,
      command,
      source: { repo: sourceRepo, path: '', commit: '', inSync: false },
      parameters: NO_PARAMETERS,
      origin: 'panel',
      successRate: 0,
      medianDurationMs: 0,
    }),
    [name, description, queue, runtime, command, sourceRepo]
  );

  const save = useMutation({
    mutationFn: (parameters: ParameterField[]) =>
      createJob({
        name: name.trim(),
        description: description.trim(),
        queue: queue.trim() || 'default',
        runtime: runtime.trim(),
        command: command.trim(),
        sourceRepo: sourceRepo.trim() || undefined,
        parameters,
      }),
    onSuccess: (job) => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      navigate(`/jobs/${encodeURIComponent(job.slug)}`);
    },
  });

  // The server requires all three; checking here keeps the user from losing an
  // authored schema to a round trip that was never going to succeed.
  const missing = !name.trim() || !runtime.trim() || !command.trim();
  const saveError = save.isError ? (save.error as Error).message : undefined;

  // The workbench holds the authored fields, so the save button lives in its
  // footer where it can be handed the current schema.
  const footer = (fields: ParameterField[]) => (
    <div className='flex flex-wrap items-center gap-3 border-t border-border pt-4'>
      <Button
        type='button'
        disabled={missing || save.isPending}
        onClick={() => save.mutate(fields)}
      >
        <Save className='mr-1.5 h-4 w-4' />
        {save.isPending ? 'Saving…' : 'Save template'}
      </Button>
      <Link to='/jobs' className='text-sm text-fg-muted hover:text-fg'>
        Cancel
      </Link>
      {missing && (
        <span className='text-sm text-fg-muted'>
          Name, runtime, and command are required.
        </span>
      )}
    </div>
  );

  return (
    <>
      <Link
        to='/jobs'
        className='mb-4 inline-flex items-center gap-1.5 text-sm text-fg-muted hover:text-fg'
      >
        <ArrowLeft className='h-4 w-4' /> Jobs
      </Link>

      <PageHeader
        title='New template'
        description='Author a template and save it to the panel. It is not in Git — export the YAML from the schema editor and commit it if you want Git to own it.'
      />

      <Card className='mb-6 border-border bg-surface p-4 sm:p-5'>
        <h2 className='font-mono text-xs uppercase tracking-wider text-fg-muted'>Template</h2>
        <div className='mt-3 grid gap-4 sm:grid-cols-2'>
          <label className='block'>
            <span className='mb-1 block text-sm font-medium text-fg'>
              Name <span className='text-danger-fg'>*</span>
            </span>
            <Input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder='Nightly rollup'
            />
          </label>
          <label className='block'>
            <span className='mb-1 block text-sm font-medium text-fg'>Queue</span>
            <Input value={queue} onChange={(e) => setQueue(e.target.value)} placeholder='default' />
          </label>
          <label className='block'>
            <span className='mb-1 block text-sm font-medium text-fg'>
              Runtime <span className='text-danger-fg'>*</span>
            </span>
            <Input
              value={runtime}
              onChange={(e) => setRuntime(e.target.value)}
              placeholder='python:3.12-slim'
            />
          </label>
          <label className='block'>
            <span className='mb-1 block text-sm font-medium text-fg'>Source repo</span>
            <Input
              value={sourceRepo}
              onChange={(e) => setSourceRepo(e.target.value)}
              placeholder='https://github.com/acme/etl (optional)'
            />
          </label>
          <label className='block sm:col-span-2'>
            <span className='mb-1 block text-sm font-medium text-fg'>
              Command <span className='text-danger-fg'>*</span>
            </span>
            <Input
              value={command}
              onChange={(e) => setCommand(e.target.value)}
              placeholder='python -m etl.rollup'
            />
          </label>
          <label className='block sm:col-span-2'>
            <span className='mb-1 block text-sm font-medium text-fg'>Description</span>
            <Input
              value={description}
              onChange={(e) => setDescription(e.target.value)}
              placeholder='What this job does'
            />
          </label>
        </div>
      </Card>

      {saveError && (
        <Alert tone='danger' className='mb-4'>
          {saveError}
        </Alert>
      )}

      <BuildWorkbench
        job={draftJob}
        builds={[]}
        authorOnly
        footer={footer}
        onRun={() => undefined}
        running={false}
      />
    </>
  );
}
