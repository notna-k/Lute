import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

export interface ChipProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  /** Drives the pressed styling and `aria-pressed`. */
  selected?: boolean;
}

/**
 * A standalone toggle, unlike SegmentedControl's joined row. Used where the
 * options are shortcuts rather than a filter — "start from defaults", "start
 * from build #412", a job template.
 */
export const Chip = forwardRef<HTMLButtonElement, ChipProps>(function Chip(
  { selected, className, type = 'button', children, ...rest },
  ref
) {
  return (
    <button
      ref={ref}
      type={type}
      aria-pressed={selected}
      className={cn(
        'inline-flex h-[26px] items-center gap-[7px] whitespace-nowrap border px-2.5 text-xs transition-colors',
        selected
          ? 'border-fg bg-bg-subtle text-fg'
          : 'border-border text-fg-muted hover:border-border-strong hover:text-fg',
        className
      )}
      {...rest}
    >
      {children}
    </button>
  );
});
