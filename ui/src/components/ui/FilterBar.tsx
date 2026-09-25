// The shared filter bar for list pages: search, scope, facets and sort, plus a chip row once anything is filtered.
import { useEffect, useRef, type ReactNode } from 'react';
import { ListFilter, Search, X } from 'lucide-react';
import { cn } from '@/lib/cn';
import { FacetMenu, type FacetConfig } from './FacetMenu';
import { Input, NativeSelect } from './Input';
import { Kbd } from './Kbd';

interface ScopeOption<T extends string> {
  value: T;
  label: string;
  count?: number;
  title?: string;
}

interface SearchConfig {
  value: string;
  onChange: (value: string) => void;
  placeholder: string;
  /** Accessible name, e.g. "Search jobs". */
  label: string;
  /** The dimension the chip names, e.g. "name". Defaults to "search". */
  chipLabel?: string;
}

interface FilterBarProps<S extends string = string> {
  search?: SearchConfig;
  /** The one state question a page asks, rendered as a single-select facet. */
  scope?: {
    /** The dimension, lowercase ("state", "result"). Names the chip too. */
    label: string;
    /** The unfiltered choice, e.g. "Any state". */
    allLabel: string;
    allCount?: number;
    value: S;
    onChange: (value: S) => void;
    /** The first option is the unfiltered one. */
    options: ScopeOption<S>[];
    icon?: ReactNode;
  };
  facets?: FacetConfig[];
  sort?: {
    value: string;
    onChange: (value: string) => void;
    options: { value: string; label: string }[];
  };
  /** Omit `shown` when filtering is server-side, or a paginated "25/84" reads as a filter. */
  count?: { shown?: number; total: number; noun: string };
  /** Trailing controls, right-aligned. */
  actions?: ReactNode;
  /** Replaces the default reset when the page has more to undo (a page number, a cursor). */
  onReset?: () => void;
  className?: string;
}

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
  const scopeDefault = scope?.options[0]?.value;
  const scopeActive = Boolean(scope && scope.value !== scopeDefault);
  const searchActive = Boolean(search?.value.trim());
  const anyActive = scopeActive || searchActive || facets.some((f) => f.values.length > 0);
  const partial = count && count.shown !== undefined && count.shown !== count.total;

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
    <div
      role='toolbar'
      aria-label='Filters'
      className={cn('flex shrink-0 flex-col border-b border-border px-7 max-md:px-4', className)}
    >
      <div className='flex flex-wrap items-center gap-2 py-[var(--density-toolbar-y)]'>
        {search && <FilterSearch search={search} />}

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
              title={partial ? `${count.shown} of ${count.total} ${count.noun} shown` : undefined}
            >
              {partial
                ? `${count.shown}/${count.total} ${count.noun}`
                : `${count.total} ${count.noun}`}
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
    </div>
  );
}

/** The search box; `f` focuses it, as `/` opens the palette. */
function FilterSearch({ search }: { search: SearchConfig }) {
  const ref = useRef<HTMLInputElement>(null);

  useEffect(() => {
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
      ref.current?.focus();
      ref.current?.select();
    }
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, []);

  // The icon-bearing Input grows to its container, so the width lives on the wrapper.
  return (
    <div className='w-[300px] max-w-full shrink-0'>
      <Input
        ref={ref}
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
                ref.current?.focus();
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
  );
}

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
