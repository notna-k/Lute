import { useMemo, useState, type ReactNode } from 'react';
import { Popover, PopoverButton, PopoverPanel } from '@headlessui/react';
import { Check, ChevronDown, Search } from 'lucide-react';
import { cn } from '@/lib/cn';
import { Input } from './Input';

export interface FacetOption {
  value: string;
  /** Shown instead of the raw value; the value is still what is filtered on. */
  label?: string;
  count?: number;
}

export interface FacetConfig {
  id: string;
  /** The dimension, lowercase: "queue", "folder", "label". Names the chip too. */
  label: string;
  /** The unfiltered state, e.g. "All queues". */
  allLabel: string;
  /** Selected values; empty means the facet is off. */
  values: string[];
  /** `null` when the page could not fetch them: the facet falls back to a text box. */
  options: FacetOption[] | null;
  onChange: (values: string[]) => void;
  /** Default true. Single-select facets close on pick and replace the value. */
  multiple?: boolean;
  /** Single-select only: the row that stands for "no filter". */
  clearOption?: { label: string; count?: number };
  icon?: ReactNode;
  menuWidth?: number;
}

const FILTER_CONTROL =
  'inline-flex h-[30px] shrink-0 items-center gap-1.5 whitespace-nowrap border px-2.5 font-mono text-[12.5px] transition-colors';

const optionLabel = (o: FacetOption) => o.label ?? o.value;

/** Distinct values with how many rows carry each, sorted by value. */
export function facetOptions(
  values: (string | null | undefined)[],
  label?: (value: string) => string,
): FacetOption[] {
  const counts = new Map<string, number>();
  for (const v of values) {
    if (!v) continue;
    counts.set(v, (counts.get(v) ?? 0) + 1);
  }
  return [...counts.entries()]
    .map(([value, count]) => ({ value, count, label: label?.(value) }))
    .sort((a, b) => a.value.localeCompare(b.value));
}

/** "release", or "release +2" once a facet holds more than it can show. */
function facetSummary(facet: FacetConfig): string {
  const [first, ...rest] = facet.values;
  const label = facet.options?.find((o) => o.value === first) ?? { value: first };
  return rest.length ? `${optionLabel(label)} +${rest.length}` : optionLabel(label);
}

/** One facet as a dropdown of its own values, since matching is exact. */
export function FacetMenu({ facet }: { facet: FacetConfig }) {
  const { options, values } = facet;
  const active = values.length > 0;

  if (options === null) {
    return (
      <Input
        value={values[0] ?? ''}
        onChange={(e) => facet.onChange(e.target.value ? [e.target.value] : [])}
        placeholder={facet.allLabel}
        aria-label={`Filter by ${facet.label}`}
        className='w-[170px]'
      />
    );
  }

  if (options.length === 0) {
    return (
      <span
        className={cn(FILTER_CONTROL, 'border-border text-fg-subtle opacity-60')}
        title={`No ${facet.label} values recorded yet`}
      >
        {facet.icon}
        {facet.allLabel}
      </span>
    );
  }

  return (
    <Popover className='relative shrink-0'>
      <PopoverButton
        className={cn(
          FILTER_CONTROL,
          'focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg',
          active
            ? 'border-fg bg-bg-subtle text-fg'
            : 'border-border text-fg-muted hover:border-border-strong hover:text-fg',
        )}
        aria-label={`Filter by ${facet.label}`}
      >
        {facet.icon}
        {active ? (
          <>
            <span className='text-fg-subtle'>{facet.label}:</span>
            <span className='max-w-[140px] truncate'>{facetSummary(facet)}</span>
          </>
        ) : (
          facet.allLabel
        )}
        <ChevronDown className='h-3.5 w-3.5 text-fg-subtle' aria-hidden />
      </PopoverButton>

      <PopoverPanel
        transition
        className='absolute left-0 z-40 mt-1 border border-border bg-bg-elevated shadow-popover transition duration-100 ease-out data-[closed]:opacity-0 data-[leave]:duration-75 data-[leave]:ease-in'
        style={{ width: facet.menuWidth ?? 248 }}
      >
        {({ close }) => <FacetPanel facet={facet} options={options} close={close} />}
      </PopoverPanel>
    </Popover>
  );
}

/** The open menu. Its own component so the find box resets whenever it unmounts. */
function FacetPanel({
  facet,
  options,
  close,
}: {
  facet: FacetConfig;
  options: FacetOption[];
  close: () => void;
}) {
  const { values, multiple = true } = facet;
  const [needle, setNeedle] = useState('');

  const visible = useMemo(() => {
    const n = needle.trim().toLowerCase();
    if (!n) return options;
    return options.filter(
      (o) => o.value.toLowerCase().includes(n) || (o.label ?? '').toLowerCase().includes(n),
    );
  }, [options, needle]);

  function clear() {
    facet.onChange([]);
    close();
  }

  function toggle(value: string) {
    if (!multiple) {
      facet.onChange(values[0] === value ? [] : [value]);
      close();
      return;
    }
    facet.onChange(values.includes(value) ? values.filter((v) => v !== value) : [...values, value]);
  }

  return (
    <>
      {options.length > 7 && (
        <div className='border-b border-border-subtle p-1.5'>
          <Input
            autoFocus
            value={needle}
            onChange={(e) => setNeedle(e.target.value)}
            placeholder={`Find a ${facet.label}…`}
            aria-label={`Find a ${facet.label}`}
            leftIcon={<Search className='h-3.5 w-3.5' />}
            className='h-[26px] border-transparent bg-transparent'
            onKeyDown={(e) => {
              if (e.key === 'Enter' && visible.length) {
                e.preventDefault();
                toggle(visible[0].value);
              }
            }}
          />
        </div>
      )}

      <div
        role={multiple ? 'group' : 'radiogroup'}
        aria-label={facet.allLabel}
        className='scrollbar-thin max-h-[264px] overflow-y-auto py-1'
      >
        {!multiple && facet.clearOption && !needle && (
          <MenuRow
            selected={values.length === 0}
            multiple={false}
            label={facet.clearOption.label}
            count={facet.clearOption.count}
            onClick={clear}
          />
        )}
        {visible.length === 0 ? (
          <p className='px-3 py-2 text-xs text-fg-subtle'>No match.</p>
        ) : (
          visible.map((o) => (
            <MenuRow
              key={o.value}
              selected={values.includes(o.value)}
              multiple={multiple}
              label={optionLabel(o)}
              count={o.count}
              onClick={() => toggle(o.value)}
            />
          ))
        )}
      </div>

      {/* A single-select menu already carries its own "any" row. */}
      {values.length > 0 && (multiple || !facet.clearOption) && (
        <div className='border-t border-border-subtle p-1'>
          <button
            type='button'
            onClick={clear}
            className='w-full px-2 py-1 text-left text-xs text-fg-muted hover:text-fg'
          >
            Clear {facet.label}
          </button>
        </div>
      )}
    </>
  );
}

/** A checkbox row widens the list; a tick-only row replaces the pick. */
function MenuRow({
  selected,
  multiple,
  label,
  count,
  onClick,
}: {
  selected: boolean;
  multiple: boolean;
  label: string;
  count?: number;
  onClick: () => void;
}) {
  return (
    <button
      type='button'
      role={multiple ? 'checkbox' : 'radio'}
      aria-checked={selected}
      onClick={onClick}
      className={cn(
        'flex w-full items-center gap-2 px-2.5 py-1 text-left text-xs hover:bg-surface-hover hover:text-fg',
        selected ? 'text-fg' : 'text-fg-muted',
      )}
    >
      {multiple ? (
        <span
          className={cn(
            'grid h-3 w-3 shrink-0 place-items-center border',
            selected ? 'border-fg bg-fg text-bg' : 'border-border-strong',
          )}
        >
          {selected && <Check className='h-2.5 w-2.5' aria-hidden />}
        </span>
      ) : (
        <span className='grid h-3 w-3 shrink-0 place-items-center'>
          {selected && <Check className='h-3 w-3' aria-hidden />}
        </span>
      )}
      <span className='flex-1 truncate font-mono'>{label}</span>
      {typeof count === 'number' && (
        <span className='font-mono text-[11px] text-fg-subtle tabular-nums'>{count}</span>
      )}
    </button>
  );
}
