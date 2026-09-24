/**
 * The filter bar every list page wears.
 *
 * Jobs, Builds and Workers ask the same three questions in the same order —
 * *which ones* (search), *in what state* (a scope), *narrowed how* (facets) —
 * so they get one control rather than three toolbars that drifted apart. A page
 * supplies the vocabulary; the layout, the keyboard, the active-filter summary
 * and the empty states are shared.
 *
 * Two rows, and the second only appears once something is filtered:
 *
 *   [ search ]  [ scope ]  [ facet ▾ ] [ facet ▾ ]  [ sort ▾ ]   12/48  [ ⟳ ]
 *   Filters  name: api ×   queue: release ×   Reset
 *
 * The chip row exists because a narrowed list looks exactly like a short one.
 * With it, "why am I only seeing four builds" is answered without opening a
 * single menu, and each reason can be dropped on its own.
 */
import { Fragment, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { Popover, Transition } from '@headlessui/react';
import { Check, ChevronDown, ListFilter, Search, X } from 'lucide-react';
import { cn } from '@/lib/cn';
import { Button } from './Button';
import { EmptyState } from './EmptyState';
import { Input, NativeSelect } from './Input';
import { Kbd } from './Kbd';
import { Toolbar } from './Toolbar';

/* ------------------------------------------------------------------ types */

export interface FacetOption {
  value: string;
  /** Shown instead of the raw value; the value is still what is filtered on. */
  label?: string;
  /** How many rows carry this value — the difference between a guess and a pick. */
  count?: number;
}

export interface FacetConfig {
  id: string;
  /** The dimension, lowercase: "queue", "folder", "label". Used in chips too. */
  label: string;
  /** The unfiltered state, e.g. "All queues". */
  allLabel: string;
  /** Currently selected values. Empty means the facet is off. */
  values: string[];
  /**
   * The values worth offering, `null` when the page could not find out — the
   * facet then degrades to a text box rather than to an empty menu.
   */
  options: FacetOption[] | null;
  onChange: (values: string[]) => void;
  /** Default true. Single-select facets close on pick and replace the value. */
  multiple?: boolean;
  /**
   * Single-select only: the row that stands for "no filter", so the menu reads
   * as a complete set of choices rather than as a checkbox with a Clear link.
   */
  clearOption?: { label: string; count?: number };
  icon?: ReactNode;
  /** Width of the menu; widen it for long values such as image references. */
  menuWidth?: number;
}

export interface ScopeOption<T extends string> {
  value: T;
  label: string;
  count?: number;
  title?: string;
}

export interface FilterBarProps<S extends string = string> {
  search?: {
    value: string;
    onChange: (value: string) => void;
    placeholder: string;
    /** Accessible name, e.g. "Search jobs". */
    label: string;
    /** The dimension the chip names, e.g. "name". Defaults to "search". */
    chipLabel?: string;
  };
  /**
   * The one state question a page asks — a build's result, a job's health, a
   * worker's availability. A dropdown like the facets rather than a row of
   * buttons: the states are mutually exclusive and there are more of them on
   * some pages than on others, and a bar whose width depends on the page reads
   * as three designs rather than one.
   */
  scope?: {
    /** The dimension, lowercase — "state", "result". Names the chip too. */
    label: string;
    /** The unfiltered choice, e.g. "Any state". */
    allLabel: string;
    /** Total behind the unfiltered choice, shown beside it. */
    allCount?: number;
    value: S;
    onChange: (value: S) => void;
    /** The first option is the unfiltered one; picking it clears the scope. */
    options: ScopeOption<S>[];
    icon?: ReactNode;
  };
  facets?: FacetConfig[];
  sort?: {
    value: string;
    onChange: (value: string) => void;
    options: { value: string; label: string }[];
  };
  /**
   * The tally on the right: "12/48 jobs", or "48 jobs" when nothing is cut.
   * Leave `shown` out where the page filters server-side and the total is
   * already the matching count — a paginated "25/84" would read as a filter.
   */
  count?: { shown?: number; total: number; noun: string };
  /** Trailing controls — refresh, density, an export. Right-aligned. */
  actions?: ReactNode;
  /**
   * Clears everything the page considers a filter. Defaults to clearing the
   * search, the facets and the scope; pass one when the page has more to undo
   * (a page number, a stored cursor).
   */
  onReset?: () => void;
  className?: string;
}

/* --------------------------------------------------------------- controls */

const CONTROL =
  'inline-flex h-[30px] shrink-0 items-center gap-1.5 whitespace-nowrap border px-2.5 font-mono text-[12.5px] transition-colors';

const optionLabel = (o: FacetOption) => o.label ?? o.value;

/**
 * Distinct values with how many rows carry each — what a facet menu wants.
 * A value holding one row is worth knowing about before picking it.
 */
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
  const label = facet.options?.find((o) => o.value === first) ?? ({ value: first } as FacetOption);
  return rest.length ? `${optionLabel(label)} +${rest.length}` : optionLabel(label);
}

/**
 * One facet as a dropdown of its own values.
 *
 * Matching is exact, so a text box would be a trap: it promises recall the panel
 * can do for you. The menu shows what exists, how many rows each value holds,
 * and — past a handful of values — a box that narrows the menu itself.
 */
function FacetMenu({ facet }: { facet: FacetConfig }) {
  const { options, values, multiple = true } = facet;
  const [needle, setNeedle] = useState('');
  const active = values.length > 0;

  const visible = useMemo(() => {
    const n = needle.trim().toLowerCase();
    if (!options) return [];
    if (!n) return options;
    return options.filter(
      (o) => o.value.toLowerCase().includes(n) || (o.label ?? '').toLowerCase().includes(n),
    );
  }, [options, needle]);

  // Only when the option list could not be fetched: typing is the last resort,
  // not the empty state.
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
        className={cn(CONTROL, 'border-border text-fg-subtle opacity-60')}
        title={`No ${facet.label} values recorded yet`}
      >
        {facet.icon}
        {facet.allLabel}
      </span>
    );
  }

  const searchable = options.length > 7;
  const width = facet.menuWidth ?? 248;

  function toggle(value: string, close: () => void) {
    if (!multiple) {
      facet.onChange(values[0] === value ? [] : [value]);
      close();
      return;
    }
    facet.onChange(values.includes(value) ? values.filter((v) => v !== value) : [...values, value]);
  }

  return (
    <Popover className='relative shrink-0'>
      <Popover.Button
        className={cn(
          CONTROL,
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
      </Popover.Button>

      <Transition
        as={Fragment}
        enter='transition ease-out duration-100'
        enterFrom='opacity-0'
        enterTo='opacity-100'
        leave='transition ease-in duration-75'
        leaveFrom='opacity-100'
        leaveTo='opacity-0'
        afterLeave={() => setNeedle('')}
      >
        <Popover.Panel
          className='absolute left-0 z-40 mt-1 border border-border bg-bg-elevated shadow-popover'
          style={{ width }}
        >
          {({ close }) => (
            <>
              {searchable && (
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
                        toggle(visible[0].value, close);
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
                    onClick={() => {
                      facet.onChange([]);
                      close();
                    }}
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
                      onClick={() => toggle(o.value, close)}
                    />
                  ))
                )}
              </div>

              {/* A single-select menu already carries its own "any" row. */}
              {active && (multiple || !facet.clearOption) && (
                <div className='border-t border-border-subtle p-1'>
                  <button
                    type='button'
                    onClick={() => {
                      facet.onChange([]);
                      close();
                    }}
                    className='w-full px-2 py-1 text-left text-xs text-fg-muted hover:text-fg'
                  >
                    Clear {facet.label}
                  </button>
                </div>
              )}
            </>
          )}
        </Popover.Panel>
      </Transition>
    </Popover>
  );
}

/**
 * One line of a filter menu. A multi-select row carries a checkbox, a
 * single-select one only a tick — the marker is what tells the operator whether
 * picking a second value widens the list or replaces the first.
 */
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

/** One reason the list is shorter than it could be, and the way to drop it. */
function FilterChip({
  label,
  value,
  onRemove,
}: {
  label: string;
  value: string;
  onRemove: () => void;
}) {
  return (
    <span className='inline-flex h-[22px] items-center gap-1.5 border border-border bg-bg-subtle pl-2 pr-1 font-mono text-[11.5px] text-fg'>
      <span className='text-fg-subtle'>{label}:</span>
      <span className='max-w-[180px] truncate'>{value}</span>
      <button
        type='button'
        onClick={onRemove}
        aria-label={`Remove the ${label} filter`}
        className='grid h-[15px] w-[15px] place-items-center text-fg-subtle transition-colors hover:bg-surface-active hover:text-fg'
      >
        <X className='h-3 w-3' aria-hidden />
      </button>
    </span>
  );
}

/**
 * What every list page shows once its filters exclude everything.
 *
 * Shared so the three pages cannot disagree about whose fault an empty table
 * is: filtered-to-nothing reads differently from having nothing, and the way
 * out — reset — belongs next to the sentence that says so.
 */
export function NoFilterMatches({ noun, onReset }: { noun: string; onReset: () => void }) {
  return (
    <div className='py-16'>
      <EmptyState
        icon={<ListFilter className='h-5 w-5' />}
        title={`No ${noun} match these filters`}
        description='Drop one of the chips above, or clear them all and start again.'
        action={
          <Button size='sm' onClick={onReset}>
            <X className='h-3.5 w-3.5' /> Reset filters
          </Button>
        }
      />
    </div>
  );
}

/* ----------------------------------------------------------------- the bar */

export function FilterBar<S extends string = string>({
  search,
  scope,
  facets = [],
  sort,
  count,
  actions,
  onReset,
  className,
}: FilterBarProps<S>) {
  const searchRef = useRef<HTMLInputElement>(null);

  // `f` focuses the box, the way `/` opens the palette: the two searches on
  // screen get one key each rather than fighting over the same one.
  useEffect(() => {
    if (!search) return;
    function onKeyDown(event: KeyboardEvent) {
      if (event.key !== 'f' || event.metaKey || event.ctrlKey || event.altKey) return;
      const target = event.target as HTMLElement | null;
      if (
        target &&
        (target.isContentEditable || ['INPUT', 'TEXTAREA', 'SELECT'].includes(target.tagName))
      ) {
        return;
      }
      event.preventDefault();
      searchRef.current?.focus();
      searchRef.current?.select();
    }
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [search]);

  const scopeDefault = scope?.options[0]?.value;
  const scopeActive = Boolean(scope && scope.value !== scopeDefault);
  const searchActive = Boolean(search?.value.trim());
  const facetActive = facets.some((f) => f.values.length > 0);
  const anyActive = scopeActive || searchActive || facetActive;

  function reset() {
    if (onReset) {
      onReset();
      return;
    }
    search?.onChange('');
    if (scope && scopeDefault !== undefined) scope.onChange(scopeDefault);
    facets.forEach((f) => f.values.length && f.onChange([]));
  }

  return (
    <Toolbar className={cn('flex-col items-stretch gap-0 py-0', className)} aria-label='Filters'>
      <div className='flex flex-wrap items-center gap-2 py-[var(--density-toolbar-y)]'>
        {/* The icon-bearing Input grows to its container, so the width lives
            here rather than on the control itself. */}
        {search && (
          <div className='w-[300px] max-w-full shrink-0'>
            <Input
              ref={searchRef}
              type='search'
              autoComplete='off'
              spellCheck={false}
              value={search.value}
              aria-label={search.label}
              placeholder={search.placeholder}
              onChange={(e) => search.onChange(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === 'Escape' && search.value) {
                  e.preventDefault();
                  search.onChange('');
                }
              }}
              leftIcon={<Search className='h-3.5 w-3.5' />}
              rightIcon={
                search.value ? (
                  <button
                    type='button'
                    aria-label='Clear the search'
                    onClick={() => {
                      search.onChange('');
                      searchRef.current?.focus();
                    }}
                    className='grid h-4 w-4 place-items-center text-fg-subtle hover:text-fg'
                  >
                    <X className='h-3.5 w-3.5' aria-hidden />
                  </button>
                ) : (
                  <Kbd className='pointer-events-none'>f</Kbd>
                )
              }
              className='[&::-webkit-search-cancel-button]:hidden'
            />
          </div>
        )}

        {scope && scopeDefault !== undefined && (
          <FacetMenu
            facet={{
              id: 'scope',
              label: scope.label,
              allLabel: scope.allLabel,
              icon: scope.icon,
              multiple: false,
              clearOption: { label: scope.allLabel, count: scope.allCount },
              values: scope.value === scopeDefault ? [] : [scope.value],
              options: scope.options
                .filter((o) => o.value !== scopeDefault)
                .map((o) => ({ value: o.value, label: o.label, count: o.count })),
              onChange: (values) => scope.onChange((values[0] as S | undefined) ?? scopeDefault),
            }}
          />
        )}

        {facets.map((facet) => (
          <FacetMenu key={facet.id} facet={facet} />
        ))}

        {sort && (
          <NativeSelect
            value={sort.value}
            aria-label='Sort order'
            className='w-[168px] shrink-0 text-fg-muted'
            onChange={(e) => sort.onChange(e.target.value)}
          >
            {sort.options.map((o) => (
              <option key={o.value} value={o.value}>
                {o.label}
              </option>
            ))}
          </NativeSelect>
        )}

        <div className='ml-auto flex shrink-0 items-center gap-2'>
          {count && (
            <span
              className='font-mono text-[11.5px] text-fg-subtle tabular-nums'
              title={
                count.shown === undefined || count.shown === count.total
                  ? undefined
                  : `${count.shown} of ${count.total} ${count.noun} shown`
              }
            >
              {count.shown === undefined || count.shown === count.total
                ? `${count.total} ${count.noun}`
                : `${count.shown}/${count.total} ${count.noun}`}
            </span>
          )}
          {actions}
        </div>
      </div>

      {anyActive && (
        <div className='flex flex-wrap items-center gap-1.5 border-t border-border-subtle py-2'>
          <span className='caption inline-flex items-center gap-1.5'>
            <ListFilter className='h-3.5 w-3.5' aria-hidden />
            filters
          </span>
          {searchActive && search && (
            <FilterChip
              label={search.chipLabel ?? 'search'}
              value={search.value}
              onRemove={() => search.onChange('')}
            />
          )}
          {scopeActive && scope && (
            <FilterChip
              label={scope.label}
              value={scope.options.find((o) => o.value === scope.value)?.label ?? scope.value}
              onRemove={() => scopeDefault !== undefined && scope.onChange(scopeDefault)}
            />
          )}
          {facets.flatMap((facet) =>
            facet.values.map((value) => (
              <FilterChip
                key={`${facet.id}:${value}`}
                label={facet.label}
                value={facet.options?.find((o) => o.value === value)?.label ?? value}
                onRemove={() => facet.onChange(facet.values.filter((v) => v !== value))}
              />
            )),
          )}
          <button
            type='button'
            onClick={reset}
            className='ml-1 text-[11.5px] text-fg-muted underline-offset-[3px] hover:text-fg hover:underline'
          >
            Reset
          </button>
        </div>
      )}
    </Toolbar>
  );
}
