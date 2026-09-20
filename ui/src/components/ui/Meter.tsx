import { cn } from '@/lib/cn';

export interface MeterProps {
  /** 0..1. Values above `hotAt` switch to the warning tint. */
  value: number;
  hotAt?: number;
  /** Hide the trailing percentage when a column already labels it. */
  showValue?: boolean;
  className?: string;
  label?: string;
}

/**
 * A utilisation bar the width of a table cell. Deliberately unlabelled axes:
 * it answers "is this machine near its limit" at a glance, and the exact figure
 * sits next to it as text.
 */
export function Meter({ value, hotAt = 0.75, showValue = true, className, label }: MeterProps) {
  const ratio = Math.max(0, Math.min(1, value));
  const hot = ratio > hotAt;
  return (
    <span
      className={cn('inline-flex items-center gap-[7px]', className)}
      role='meter'
      aria-valuenow={Math.round(ratio * 100)}
      aria-valuemin={0}
      aria-valuemax={100}
      aria-label={label}
    >
      <span className='relative block h-1 w-14 bg-border'>
        <span
          className={cn('absolute inset-y-0 left-0', hot ? 'bg-warning' : 'bg-fg-muted')}
          style={{ width: `${ratio * 100}%` }}
        />
      </span>
      {showValue && (
        <span className='w-[30px] text-right font-mono text-[11.5px] text-fg-muted tabular-nums'>
          {Math.round(ratio * 100)}%
        </span>
      )}
    </span>
  );
}

export interface SlotsProps {
  /** How many execution slots the worker or queue lane offers. */
  total: number;
  used: number;
  className?: string;
}

/** Discrete capacity: one cell per slot, filled while occupied. */
export function Slots({ total, used, className }: SlotsProps) {
  return (
    <span
      className={cn('inline-flex items-center gap-0.5 align-middle', className)}
      aria-label={`${used} of ${total} slots busy`}
    >
      {Array.from({ length: Math.max(total, 0) }, (_, i) => (
        <span
          key={i}
          aria-hidden
          className={cn(
            'h-[11px] w-[9px] border border-fg-subtle',
            i < used && 'border-warning bg-warning',
          )}
        />
      ))}
    </span>
  );
}
