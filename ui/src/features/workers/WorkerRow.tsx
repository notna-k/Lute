import { Link } from "react-router-dom";
import { MoreVertical, Power, Trash2 } from "lucide-react";
import { Menu, Transition } from "@headlessui/react";
import { Fragment } from "react";
import type { Worker } from "@/types";
import { Badge } from "@/components/ui";
import { cn } from "@/lib/cn";
import { statusTone, workerInitials } from "./utils";

export interface WorkerRowProps {
  worker: Worker;
  onReEnable?: (w: Worker) => void;
  onDelete?: (w: Worker) => void;
  reEnablePending?: boolean;
  deletePending?: boolean;
}

export function WorkerRow({
  worker: w,
  onReEnable,
  onDelete,
  reEnablePending,
  deletePending,
}: WorkerRowProps) {
  return (
    <div className="flex flex-col gap-3 border-b border-border px-4 py-3 transition-colors hover:bg-surface-hover first:rounded-t-lg last:rounded-b-lg last:border-0 sm:flex-row sm:items-center sm:gap-4">
      <Link
        to={`/workers/${w.id}`}
        className="flex min-w-0 flex-1 items-center gap-3"
      >
        <span className="flex h-10 w-10 shrink-0 items-center justify-center rounded-md bg-primary-subtle text-info-fg text-sm font-semibold">
          {workerInitials(w.name)}
        </span>
        <div className="min-w-0">
          <div className="flex items-center gap-2">
            <span className="truncate text-sm font-semibold text-fg">
              {w.name}
            </span>
          </div>
          <p className="truncate text-xs text-fg-muted">
            {w.description || "No description"}
          </p>
        </div>
      </Link>

      <div className="flex items-center gap-3">
        <Badge tone={statusTone(w.status)} dot>
          {w.status}
        </Badge>
        {w.status === "dead" && onReEnable && (
          <button
            type="button"
            onClick={() => onReEnable(w)}
            disabled={reEnablePending}
            className="inline-flex items-center gap-1 rounded-md border border-border bg-surface px-2.5 py-1 text-xs font-medium text-fg hover:bg-surface-hover disabled:opacity-50"
          >
            <Power className="h-3.5 w-3.5" />
            {reEnablePending ? "Re-enabling…" : "Re-enable"}
          </button>
        )}
        {onDelete && (
          <Menu as="div" className="relative">
            <Menu.Button
              className="inline-flex h-8 w-8 items-center justify-center rounded-md text-fg-muted hover:bg-surface-hover hover:text-fg"
              aria-label={`More actions for ${w.name}`}
            >
              <MoreVertical className="h-4 w-4" />
            </Menu.Button>
            <Transition
              as={Fragment}
              enter="transition ease-out duration-100"
              enterFrom="opacity-0 translate-y-1"
              enterTo="opacity-100 translate-y-0"
              leave="transition ease-in duration-75"
              leaveFrom="opacity-100"
              leaveTo="opacity-0"
            >
              <Menu.Items className="absolute right-0 z-30 mt-1 w-40 origin-top-right rounded-md border border-border bg-surface py-1 shadow-popover focus:outline-none">
                <Menu.Item>
                  {({ active }) => (
                    <Link
                      to={`/workers/${w.id}`}
                      className={cn(
                        "block px-3 py-2 text-sm text-fg",
                        active && "bg-surface-hover"
                      )}
                    >
                      Manage
                    </Link>
                  )}
                </Menu.Item>
                <Menu.Item disabled={deletePending}>
                  {({ active }) => (
                    <button
                      type="button"
                      onClick={() => onDelete(w)}
                      className={cn(
                        "flex w-full items-center gap-2 px-3 py-2 text-sm text-danger",
                        active && "bg-danger-subtle"
                      )}
                    >
                      <Trash2 className="h-4 w-4" />
                      Delete
                    </button>
                  )}
                </Menu.Item>
              </Menu.Items>
            </Transition>
          </Menu>
        )}
      </div>
    </div>
  );
}
