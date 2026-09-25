/**
 * How a definition relates to Git, and the ways out of drift.
 *
 * Git is the source of truth, but the panel may edit a definition or create
 * one. Anything that differs from Git gets a yellow dot; its config can be
 * viewed and copied — one job or all of them — to commit it, which makes the
 * panel's version canonical on the next sync.
 */
import { useState } from 'react';
import { Menu, MenuButton, MenuItem, MenuItems } from '@headlessui/react';
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Check,
  Copy,
  Download,
  FileArchive,
  FileCode2,
  GitBranch,
  MoreVertical,
  Undo2,
} from 'lucide-react';
import { Badge } from '@/components/ui/Badge';
import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import { IconButton } from '@/components/ui/IconButton';
import { Spinner } from '@/components/ui/Spinner';
import { cn } from '@/lib/cn';
import { downloadFile, downloadYaml } from '@/features/params/yaml';
import { exportJob, exportJobs, exportJobsZip, revertJob } from '@/services/jobDefService';
import type { GitState, JobDefinition } from '@/types/jobs';

const GIT_STATE_LABEL: Record<Exclude<GitState, 'synced'>, string> = {
  modified: 'Changed in the panel',
  manual: 'Created in the panel',
  removed: 'Removed from Git',
};

/** The state as a labelled chip, for a job's header. */
export function GitStateBadge({ state }: { state: GitState }) {
  if (state === 'synced') {
    return (
      <Badge tone='neutral' size='sm' title='Matches its YAML file'>
        <GitBranch className='h-3 w-3' /> in sync
      </Badge>
    );
  }
  return (
    <Badge tone='warning' size='sm' dot title={GIT_STATE_LABEL[state]}>
      {GIT_STATE_LABEL[state]}
    </Badge>
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
  // The zip is a second rendering of the same definitions — one file per job,
  // at its path in Git — so it is built by the server, not from the stream
  // shown here.
  const zip = useMutation({
    mutationFn: exportJobsZip,
    onSuccess: (blob) => downloadFile(blob, 'jobdefs.zip'),
  });
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
          : 'Every definition as one YAML stream. Each document is headed by its file — paste it into the job-definitions repo as is, or take the zip, which is already split into one file per job at those paths.'
      }
      footer={
        <>
          {!slug && (
            <Button
              variant='secondary'
              type='button'
              loading={zip.isPending}
              onClick={() => zip.mutate()}
            >
              <FileArchive className='mr-1.5 h-4 w-4' /> Download .zip
            </Button>
          )}
          <Button
            variant='secondary'
            type='button'
            disabled={!yaml}
            onClick={() => yaml && downloadYaml(yaml.trimEnd(), `${slug ?? 'jobdefs'}.yaml`)}
          >
            <Download className='mr-1.5 h-4 w-4' /> {slug ? 'Download' : 'Download .yaml'}
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
        <p className='text-sm text-danger'>{(error as Error).message}</p>
      ) : (
        <>
          {zip.error && (
            <p className='mb-2 text-sm text-danger'>
              Could not build the zip: {(zip.error as Error).message}
            </p>
          )}
          <pre className='max-h-[60vh] overflow-auto border border-log-line bg-log-bg p-3 font-mono text-xs leading-relaxed text-log-fg'>
            {yaml}
          </pre>
        </>
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
        <MenuButton as={IconButton} size='sm' label={`More actions for ${job.name}`}>
          <MoreVertical className='h-4 w-4' />
        </MenuButton>
        <MenuItems
          transition
          className='absolute right-0 z-30 mt-1 w-48 origin-top-right border border-border bg-surface py-1 text-left shadow-popover focus:outline-none transition duration-100 ease-out data-[closed]:translate-y-1 data-[closed]:opacity-0 data-[leave]:duration-75 data-[leave]:ease-in'
        >
          {items.map((item) => {
            const Icon = item.icon;
            return (
              <MenuItem key={item.label}>
                <button
                  type='button'
                  onClick={item.onClick}
                  className='flex w-full items-center gap-2 px-3 py-1.5 text-[13px] text-fg data-[focus]:bg-surface-hover'
                >
                  <Icon className='h-4 w-4 text-fg-muted' />
                  {item.label}
                </button>
              </MenuItem>
            );
          })}
        </MenuItems>
      </Menu>
      <ConfigDialog open={viewing} onClose={() => setViewing(false)} slug={job.slug} />
    </>
  );
}
