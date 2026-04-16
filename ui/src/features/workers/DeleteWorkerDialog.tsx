import { Button, Dialog } from "@/components/ui";
import type { Worker } from "@/types";

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
      size="sm"
      title="Delete worker?"
      footer={
        <>
          <Button variant="ghost" onClick={onCancel} disabled={pending}>
            Cancel
          </Button>
          <Button variant="danger" onClick={onConfirm} loading={pending}>
            Delete
          </Button>
        </>
      }
    >
      {worker && (
        <div className="space-y-2 text-sm text-fg-muted">
          <p>
            Are you sure you want to delete{" "}
            <span className="font-semibold text-fg">{worker.name}</span>? This
            will remove the worker and its history. This action cannot be
            undone.
          </p>
          <p>
            If the agent is currently connected, the server will signal it to
            finish in-flight jobs and exit.
          </p>
        </div>
      )}
    </Dialog>
  );
}
