/**
 * How a definition relates to Git, and the ways out of drift.
 *
 * Git is the source of truth, but the panel may edit a definition or create
 * one. Anything that differs from Git gets a yellow dot; its config can be
 * viewed and copied — one job or all of them — to commit it, which makes the
 * panel's version canonical on the next sync.
 */
import { Fragment, useState } from 'react';
import { Menu, Transition } from '@headlessui/react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import { Check, Copy, Download, FileCode2, GitBranch, MoreVertical, Undo2 } from 'lucide-react';
import { Button, Dialog, Spinner, Tooltip } from '@/components/ui';
import { cn } from '@/lib/cn';
import { downloadYaml } from '@/features/params/yaml';
import { exportJob, exportJobs, revertJob } from '@/services/jobDefService';
import type { GitState, JobDefinition } from '@/types/jobs';

const GIT_STATE_LABEL: Record<Exclude<GitState, 'synced'>, string> = {
  modified: 'Changed in the panel',
  manual: 'Created in the panel',
  removed: 'Removed from Git',
};

/** The yellow marker on anything that differs from Git; nothing when synced. */
export function GitStateDot({ state, className }: { state: GitState; className?: string }) {
  if (state === 'synced') return null;
  const label = GIT_STATE_LABEL[state];
  return (
    <span className={cn('inline-flex', className)}>
      <Tooltip side='left' content={label}>
        <span
          tabIndex={0}
          aria-label={label}
          className='block h-3 w-3 rounded-full bg-warning ring-2 ring-bg'
        />
      </Tooltip>
    </span>
  );
}

/** The state as a labelled chip, for a job's header. */
export function GitStateBadge({ state }: { state: GitState }) {
  const chip =
    'inline-flex items-center gap-1.5 rounded border px-2 py-0.5 text-xxs font-medium uppercase tracking-wide';
  if (state === 'synced') {
    return (
      <span className={cn(chip, 'border-success/30 bg-success-subtle text-success-fg')}>
        <GitBranch className='h-3 w-3' /> in sync with Git
      </span>
    );
  }
  return (
    <span className={cn(chip, 'border-warning/30 bg-warning-subtle text-warning-fg')}>
      <span className='h-2 w-2 rounded-full bg-warning' />
      {GIT_STATE_LABEL[state]}
    </span>
  );
}

/** Copies text, and says so for a moment. */
function useCopy(): [boolean, (text: string) => Promise<void>] {
  const [copied, setCopied] = useState(false);
  async function copy(text: string) {
    await navigator.clipboard.writeText(text);
    setCopied(true);
    setTimeout(() => setCopied(false), 1500);
  }
  return [copied, copy];
}

/**
 * Shows YAML ready to commit — one definition's (`slug`) or, without one, every
 * definition's as a multi-document stream.
 */
export function ConfigDialog({
  open,
  onClose,
  slug,
}: {
  open: boolean;
  onClose: () => void;
  slug?: string;
}) {
  const [copied, copy] = useCopy();
  const {
    data: yaml,
    isLoading,
    error,
  } = useQuery({
    queryKey: ['job-yaml', slug ?? '*'],
    queryFn: () => (slug ? exportJob(slug) : exportJobs()),
    enabled: open,
    // Always the current state: this is what gets committed.
    staleTime: 0,
  });

  return (
    <Dialog
      open={open}
      onClose={onClose}
      size='lg'
      title={slug ? `${slug} · config` : 'All job definitions'}
      description={
        slug
          ? 'The YAML this job would have in Git. Commit it to make the panel’s version canonical.'
          : 'Every definition as one YAML stream. Each document is headed by its file — paste it into the job-definitions repo as is, or split it by those paths.'
      }
      footer={
        <>
          <Button
            variant='secondary'
            type='button'
            disabled={!yaml}
            onClick={() => yaml && downloadYaml(yaml.trimEnd(), `${slug ?? 'jobdefs'}.yaml`)}
          >
            <Download className='mr-1.5 h-4 w-4' /> Download
          </Button>
          <Button type='button' disabled={!yaml} onClick={() => yaml && copy(yaml)}>
            {copied ? <Check className='mr-1.5 h-4 w-4' /> : <Copy className='mr-1.5 h-4 w-4' />}
            {copied ? 'Copied' : 'Copy'}
          </Button>
        </>
      }
    >
      {isLoading ? (
        <div className='flex justify-center py-10'>
          <Spinner />
        </div>
      ) : error ? (
        <p className='text-sm text-danger-fg'>{(error as Error).message}</p>
      ) : (
        <pre className='max-h-[60vh] overflow-auto rounded-md bg-bg p-3 font-mono text-xs leading-relaxed text-fg'>
          {yaml}
        </pre>
      )}
    </Dialog>
  );
}

/** Per-job actions: view or copy its config, and revert panel edits. */
export function JobActionsMenu({ job, className }: { job: JobDefinition; className?: string }) {
  const queryClient = useQueryClient();
  const [viewing, setViewing] = useState(false);
  const [copied, copy] = useCopy();

  const revert = useMutation({
    mutationFn: () => revertJob(job.slug),
    onSuccess: () => {
      void queryClient.invalidateQueries({ queryKey: ['jobs'] });
      void queryClient.invalidateQueries({ queryKey: ['job', job.slug] });
    },
  });

  const items = [
    { label: 'View config', icon: FileCode2, onClick: () => setViewing(true) },
    {
      label: copied ? 'Copied' : 'Copy YAML',
      icon: copied ? Check : Copy,
      onClick: async () => copy(await exportJob(job.slug)),
    },
    ...(job.gitState === 'modified'
      ? [
          {
            label: 'Revert to Git',
            icon: Undo2,
            onClick: () => revert.mutate(),
          },
        ]
      : []),
  ];

  return (
    <>
      <Menu as='div' className={cn('relative', className)}>
        <Menu.Button
          className='inline-flex h-7 w-7 items-center justify-center rounded-md text-fg-muted hover:bg-surface-hover hover:text-fg'
          aria-label={`More actions for ${job.name}`}
        >
          <MoreVertical className='h-4 w-4' />
        </Menu.Button>
        <Transition
          as={Fragment}
          enter='transition ease-out duration-100'
          enterFrom='opacity-0 translate-y-1'
          enterTo='opacity-100 translate-y-0'
          leave='transition ease-in duration-75'
          leaveFrom='opacity-100'
          leaveTo='opacity-0'
        >
          <Menu.Items className='absolute right-0 z-30 mt-1 w-44 origin-top-right rounded-md border border-border bg-surface py-1 shadow-popover focus:outline-none'>
            {items.map((item) => {
              const Icon = item.icon;
              return (
                <Menu.Item key={item.label}>
                  {({ active }) => (
                    <button
                      type='button'
                      onClick={item.onClick}
                      className={cn(
                        'flex w-full items-center gap-2 px-3 py-2 text-sm text-fg',
                        active && 'bg-surface-hover'
                      )}
                    >
                      <Icon className='h-4 w-4 text-fg-muted' />
                      {item.label}
                    </button>
                  )}
                </Menu.Item>
              );
            })}
          </Menu.Items>
        </Transition>
      </Menu>
      <ConfigDialog open={viewing} onClose={() => setViewing(false)} slug={job.slug} />
    </>
  );
}
