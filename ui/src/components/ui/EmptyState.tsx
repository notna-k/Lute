import { type ReactNode } from 'react';
import { cn } from '@/lib/cn';

export interface EmptyStateProps {
  icon?: ReactNode;
  title: string;
  description?: ReactNode;
  action?: ReactNode;
  className?: string;
}

/** The nothing-here state. Unboxed, since it usually sits inside a bordered table or panel. */
export function EmptyState({ icon, title, description, action, className }: EmptyStateProps) {
  return (
    <div
      className={cn(
        'grid justify-items-center gap-2.5 px-5 py-10 text-center text-fg-subtle',
        className,
      )}
    >
      {icon}
      <b className='text-sm font-medium text-fg'>{title}</b>
      {description && <span className='max-w-md text-[13px]'>{description}</span>}
      {action}
    </div>
  );
}
