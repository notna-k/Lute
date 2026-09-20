import { Fragment, type ReactNode } from 'react';
import { cn } from '@/lib/cn';

export interface KeyValueRow {
  key: string;
  /** Rendered as-is, so a value can be a link, a marker or a secret stand-in. */
  value: ReactNode;
  /** Plain-text form for the hover title, when the value is truncated. */
  title?: string;
}

export interface KeyValueListProps {
  rows: KeyValueRow[];
  className?: string;
}

/**
 * A two-column definition list for build facts: parameters, placement, source.
 * Keys are monospaced and dimmed so the values form the readable column.
 */
export function KeyValueList({ rows, className }: KeyValueListProps) {
  return (
    <dl
      className={cn(
        'grid grid-cols-[auto_minmax(0,1fr)] gap-x-3 gap-y-1',
        className
      )}
    >
      {rows.map((row) => (
        // dt/dd are the grid items themselves, so the two columns line up
        // across every row without a wrapper per pair.
        <Fragment key={row.key}>
          <dt className='font-mono text-[11.5px] text-fg-subtle'>{row.key}</dt>
          <dd className='m-0 truncate font-mono text-xs' title={row.title}>
            {row.value}
          </dd>
        </Fragment>
      ))}
    </dl>
  );
}

export interface LabelChipsProps {
  labels?: Record<string, string>;
  /** Shown when there are no labels — "any" for a selector, "none" for a worker. */
  emptyText?: string;
  className?: string;
}

/** `key=value` routing labels, as split boxes so the key reads as the key. */
export function LabelChips({ labels, emptyText = 'any', className }: LabelChipsProps) {
  const entries = Object.entries(labels ?? {});
  if (!entries.length) {
    return <span className={cn('text-fg-subtle', className)}>{emptyText}</span>;
  }
  return (
    <span className={cn('inline-flex flex-wrap gap-1', className)}>
      {entries.map(([key, value]) => (
        <span
          key={key}
          className='inline-flex border border-border font-mono text-[11px] leading-[17px]'
        >
          <span className='border-r border-border px-[5px] text-fg-subtle'>{key}</span>
          <span className='px-[5px]'>{value}</span>
        </span>
      ))}
    </span>
  );
}
