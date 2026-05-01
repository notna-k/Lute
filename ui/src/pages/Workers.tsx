import { useState } from 'react';
import { Plus, Server } from 'lucide-react';
import {
  useDeleteWorker,
  useReEnableWorker,
  useUserWorkers,
} from '@/hooks/useWorkers';
import type { Worker } from '@/types';
import { Alert, Button, EmptyState, PageHeader } from '@/components/ui';
import { AddWorkerDialog } from '@/features/workers/AddWorkerDialog';
import { DeleteWorkerDialog } from '@/features/workers/DeleteWorkerDialog';
import { WorkerList } from '@/features/workers/WorkerList';

const Workers = () => {
  const [addOpen, setAddOpen] = useState(false);
  const [deleteTarget, setDeleteTarget] = useState<Worker | null>(null);

  const userQuery = useUserWorkers();
  const reEnable = useReEnableWorker();
  const remove = useDeleteWorker();

  const workers = userQuery.data ?? [];

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
