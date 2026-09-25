import { type ReactNode } from 'react';
import { cn } from '@/lib/cn';

interface SegmentOption<T extends string> {
  value: T;
  label: ReactNode;
  /** Trailing count, e.g. how many builds the filter would show. */
  count?: number;
  title?: string;
}

export interface SegmentedControlProps<T extends string> {
  value: T;
  options: SegmentOption<T>[];
  onChange: (value: T) => void;
  /** Names the group for screen readers, e.g. "Filter builds". */
  label: string;
  className?: string;
}

/**
 * A joined row of mutually exclusive filters — one border around the group,
 * hairlines between the segments. Used for status filters, preview modes and
 * the settings toggles, so those choices all look and behave alike.
 */
export function SegmentedControl<T extends string>({
  value,
  options,
  onChange,
  label,
  className,
}: SegmentedControlProps<T>) {
  return (
    <div
      role='group'
      aria-label={label}
      className={cn('inline-flex border border-border', className)}
    >
      {options.map((option, i) => {
        const active = option.value === value;
        return (
          <button
            key={option.value}
            type='button'
            aria-pressed={active}
            title={option.title}
            onClick={() => onChange(option.value)}
            className={cn(
              'inline-flex h-[26px] items-center gap-1.5 px-2.5 text-xs transition-colors',
              i < options.length - 1 && 'border-r border-border',
              active ? 'bg-surface-active text-fg' : 'text-fg-subtle hover:text-fg',
            )}
          >
            {option.label}
            {typeof option.count === 'number' && (
              <span className='font-mono text-[11px] text-fg-subtle tabular-nums'>
                {option.count}
              </span>
            )}
          </button>
        );
      })}
    </div>
  );
}
