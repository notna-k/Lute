import { useState, useMemo } from 'react';
import { Plus, Search } from 'lucide-react';
import {
  useDeleteWorker,
  useReEnableWorker,
  useUserWorkers,
} from '@/hooks/useWorkers';
import type { Worker } from '@/types';
import { Alert, Button, Input, PageHeader } from '@/components/ui';
import { AddWorkerDialog } from '@/features/workers/AddWorkerDialog';
import { DeleteWorkerDialog } from '@/features/workers/DeleteWorkerDialog';
import { WorkerList } from '@/features/workers/WorkerList';

function parseLabelFilter(raw: string): Record<string, string> {
  const out: Record<string, string> = {};
  raw.split(',').forEach((part) => {
    const [k, ...rest] = part.trim().split('=');
    if (k) out[k.trim()] = rest.join('=').trim();
  });
  return out;
}

function workerMatchesFilter(w: Worker, filter: Record<string, string>): boolean {
  for (const [k, v] of Object.entries(filter)) {
    if (!k) continue;
    if ((w.labels ?? {})[k] !== v) return false;
  }
  return true;
}

const Workers = () => {
  const [addOpen, setAddOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Worker | null>(null);
  const [labelFilter, setLabelFilter] = useState('');

  const userQuery = useUserWorkers();
  const reEnable = useReEnableWorker();
  const remove = useDeleteWorker();

  const parsedFilter = useMemo(() => parseLabelFilter(labelFilter), [labelFilter]);
  const workers = useMemo(() => {
    const all = userQuery.data ?? [];
    return labelFilter.trim() ? all.filter((w) => workerMatchesFilter(w, parsedFilter)) : all;
  }, [userQuery.data, labelFilter, parsedFilter]);

  const handleDeleteConfirm = () => {
    if (!deleteTarget) return;
    remove.mutate(deleteTarget.id, {
      onSuccess: () => {
        setDeleteTarget(null);
      },
    });
  };

  return (
    <>
      <PageHeader
        title='Workers'
        description='Register, monitor, and manage compute agents.'
        actions={
          <Button
            leftIcon={<Plus className='h-4 w-4' />}
            onClick={() => setAddOpen(true)}
          >
            Add worker
          </Button>
        }
      />

      <div className='mb-4 flex items-center gap-2'>
        <div className='relative flex-1 max-w-xs'>
          <Search className='absolute left-2.5 top-2.5 h-4 w-4 text-fg-muted pointer-events-none' />
          <Input
            placeholder='Filter by label, e.g. gpu=true'
            value={labelFilter}
            onChange={(e) => setLabelFilter(e.target.value)}
            className='pl-8'
          />
        </div>
        {labelFilter && (
          <button
            type='button'
            onClick={() => setLabelFilter('')}
            className='text-xs text-fg-muted hover:text-fg'
          >
            Clear
          </button>
        )}
      </div>

      {userQuery.isError && (
        <Alert tone='danger' className='mb-4'>
          Failed to load workers:{' '}
          {userQuery.error instanceof Error
            ? userQuery.error.message
            : 'Unknown error'}
        </Alert>
      )}

      <WorkerList
        workers={workers}
        loading={userQuery.isLoading}
        onReEnable={(w) =>
          reEnable.mutate(w.id, {
            onSuccess: () => userQuery.refetch(),
          })
        }
        onDelete={(w) => setDeleteTarget(w)}
        reEnablingId={
          reEnable.isPending ? (reEnable.variables as string | undefined) : undefined
        }
        deletingId={
          remove.isPending ? (remove.variables as string | undefined) : undefined
        }
        empty={
          <EmptyState
            icon={<Server className='h-5 w-5' />}
            title='No workers yet'
            description='Register your first compute agent to start running jobs.'
            action={
              <Button
                leftIcon={<Plus className='h-4 w-4' />}
                onClick={() => setAddOpen(true)}
              >
                Add your first worker
              </Button>
            }
          />
        }
      />

      <AddWorkerDialog open={addOpen} onClose={() => setAddOpen(false)} />
      <DeleteWorkerDialog
        worker={deleteTarget}
        onCancel={() => setDeleteTarget(null)}
        onConfirm={handleDeleteConfirm}
        pending={remove.isPending}
      />
    </>
  );
};

export default Workers;
