import type { ReactNode } from "react";
import type { Worker } from "@/types";
import { Card, Skeleton } from "@/components/ui";
import { WorkerRow } from "./WorkerRow";

export interface WorkerListProps {
  workers: Worker[];
  loading?: boolean;
  onReEnable?: (w: Worker) => void;
  onDelete?: (w: Worker) => void;
  reEnablingId?: string;
  deletingId?: string;
  empty: ReactNode;
}

export function WorkerList({
  workers,
  loading,
  onReEnable,
  onDelete,
  reEnablingId,
  deletingId,
  empty,
}: WorkerListProps) {
  if (loading) {
    return (
      <Card>
        <div className="divide-y divide-border">
          {Array.from({ length: 4 }).map((_, i) => (
            <div
              key={i}
              className="flex items-center gap-3 px-4 py-3"
            >
              <Skeleton className="h-10 w-10 rounded-md" />
              <div className="flex-1 space-y-1.5">
                <Skeleton className="h-4 w-1/3" />
                <Skeleton className="h-3 w-1/2" />
              </div>
              <Skeleton className="h-6 w-16 rounded-full" />
            </div>
          ))}
        </div>
      </Card>
    );
  }

  if (!workers.length) {
    return <>{empty}</>;
  }

  return (
    <Card>
      {workers.map((w) => (
        <WorkerRow
          key={w.id}
          worker={w}
          onReEnable={onReEnable}
          onDelete={onDelete}
          reEnablePending={reEnablingId === w.id}
          deletePending={deletingId === w.id}
        />
      ))}
    </Card>
  );
}
