import { Button } from '@/components/ui/Button';
import { Dialog } from '@/components/ui/Dialog';
import type { Worker } from '@/types';

interface DeleteWorkerDialogProps {
  worker: Worker | null;
  onCancel: () => void;
  onConfirm: () => void;
  pending?: boolean;
}

export function DeleteWorkerDialog({
  worker,
  onCancel,
  onConfirm,
  pending,
}: DeleteWorkerDialogProps) {
  return (
    <Dialog
      open={!!worker}
      onClose={onCancel}
      size='sm'
      title='Delete worker?'
      footer={
        <>
          <Button variant='ghost' onClick={onCancel} disabled={pending}>
            Cancel
          </Button>
          <Button variant='danger' onClick={onConfirm} loading={pending}>
            Delete
          </Button>
        </>
      }
    >
      {worker && (
        <div className='space-y-2 text-sm text-fg-muted'>
          <p>
            Delete <span className='font-semibold text-fg'>{worker.name}</span>? It finishes the
            builds it is running, takes no new ones, and then stops its container.
          </p>
          <p>
            Nothing on the host is removed: the container, its identity and its job logs stay. To
            bring it back, delete <code className='font-mono text-[12px]'>state.json</code> and
            start it with a registration token.
          </p>
        </div>
      )}
    </Dialog>
  );
}
