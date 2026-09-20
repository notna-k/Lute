import { forwardRef, type HTMLAttributes } from 'react';
import { Search } from 'lucide-react';
import { cn } from '@/lib/cn';
import { Input, NativeSelect, type InputProps } from './Input';

/**
 * The filter strip under a page header: search, segmented filters, and a result
 * count pushed to the right. Fixed height and its own bottom hairline, so it
 * reads as part of the page frame rather than as the first row of content.
 */
export const Toolbar = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function Toolbar({ className, ...props }, ref) {
    return (
      <div
        ref={ref}
        className={cn(
          'flex shrink-0 flex-wrap items-center gap-2.5 border-b border-border px-7 py-[var(--density-toolbar-y)]',
          className
        )}
        {...props}
      />
    );
  }
);

export interface SearchInputProps extends Omit<InputProps, 'leftIcon'> {
  className?: string;
}

/** A search box with the magnifier baked in, so every filter looks the same. */
export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
  function SearchInput({ className, ...props }, ref) {
    return (
      <Input
        ref={ref}
        type='search'
        autoComplete='off'
        leftIcon={<Search className='h-3.5 w-3.5' />}
        className={cn('w-[280px] max-w-full', className)}
        {...props}
      />
    );
  }
);

export interface FilterSelectProps {
  /** The dimension being filtered, e.g. "queue"; used for the accessible name. */
  label: string;
  /** The unfiltered choice, e.g. "All queues". */
  allLabel: string;
  value: string;
  /**
   * The values worth offering. Empty means there are none to filter by yet;
   * `null` means the panel could not find out, and the operator types instead.
   */
  options: string[] | null;
  onChange: (value: string) => void;
  className?: string;
}

/**
 * A toolbar filter over a closed set of values. Matching is exact, and a search
 * box would promise otherwise — so the control shows the values rather than
 * asking the operator to recall one, and goes quiet when there are none. An
 * active filter carries the stronger border, so a narrowed list is visible
 * without reading the values.
 */
export function FilterSelect({
  label,
  allLabel,
  value,
  options,
  onChange,
  className,
}: FilterSelectProps) {
  const ariaLabel = `Filter by ${label}`;

  // Only when the option list could not be fetched: typing is the last resort,
  // not the empty state.
  if (options === null) {
    return (
      <SearchInput
        value={value}
        onChange={(e) => onChange(e.target.value)}
        placeholder={allLabel}
        aria-label={ariaLabel}
        className={cn('w-[160px]', className)}
      />
    );
  }

  return (
    <NativeSelect
      value={value}
      aria-label={ariaLabel}
      disabled={options.length === 0}
      onChange={(e) => onChange(e.target.value)}
      className={cn(
        'w-[160px]',
        value ? 'border-border-strong' : 'text-fg-muted',
        className
      )}
    >
      <option value=''>{allLabel}</option>
      {options.map((o) => (
        <option key={o} value={o}>
          {o}
        </option>
      ))}
    </NativeSelect>
  );
}
