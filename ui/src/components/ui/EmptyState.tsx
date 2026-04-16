import { type ReactNode } from "react";
import { cn } from "@/lib/cn";

export interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
}

export function EmptyState({
  icon,
  title,
  description,
  action,
  className,
}: EmptyStateProps) {
  return (
    <div
      className={cn(
        "flex flex-col items-center justify-center gap-3 rounded-lg border border-dashed border-border bg-bg-subtle px-6 py-12 text-center",
        className
      )}
    >
      {icon && (
        <div className="flex h-10 w-10 items-center justify-center rounded-full bg-bg-muted text-fg-muted">
          {icon}
        </div>
      )}
      <h3 className="text-base font-semibold text-fg">{title}</h3>
      {description && (
        <p className="max-w-md text-sm text-fg-muted">{description}</p>
      )}
      {action && <div className="mt-1">{action}</div>}
    </div>
  );
}
