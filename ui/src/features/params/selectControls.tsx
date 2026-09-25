import { useState } from 'react';
import { Check, ChevronDown, Search, X } from 'lucide-react';
import { cn } from '@/lib/cn';
import { labelOf, popoverRing, useDismiss } from './controlStyles';
import { TONE_TAG } from './tones';
import type { ParamInputProps } from './types';

export function SelectInput({ field, value, onChange, invalid, disabled }: ParamInputProps) {
  const options = field.options ?? [];
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const ref = useDismiss(() => setOpen(false));
  const selected = options.find((o) => o.value === value);
  // Search only earns its place once the list scans poorly.
  const searchable = options.length > 7;
  const shown = query
    ? options.filter((o) => `${o.value} ${labelOf(o)}`.toLowerCase().includes(query.toLowerCase()))
    : options;

  return (
    <div ref={ref} className='relative'>
      <button
        type='button'
        disabled={disabled}
        onClick={() => setOpen((o) => !o)}
        aria-haspopup='listbox'
        aria-expanded={open}
        className={cn(
          'flex w-full items-center gap-2.5 rounded-md border bg-bg px-3 py-2 text-left text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-60',
          popoverRing(open, invalid),
        )}
      >
        {selected?.tone && (
          <span className={cn('rounded px-1.5 py-0.5 font-mono text-xxs', TONE_TAG[selected.tone])}>
            {selected.value}
          </span>
        )}
        <span className={cn('truncate', selected ? 'text-fg' : 'text-fg-subtle')}>
          {selected ? labelOf(selected) : 'Select…'}
        </span>
        {selected?.hint && (
          <span className='truncate text-xs text-fg-muted'>· {selected.hint}</span>
        )}
        <ChevronDown
          className={cn(
            'ml-auto h-4 w-4 shrink-0 text-fg-muted transition-transform',
            open && 'rotate-180',
          )}
        />
      </button>
      {open && (
        <div
          role='listbox'
          className='absolute z-30 mt-1.5 w-full rounded-lg border border-border bg-bg-elevated p-1.5 shadow-popover'
        >
          {searchable && (
            <div className='mb-1 flex items-center gap-2 border-b border-border-subtle px-2 pb-1.5'>
              <Search className='h-3.5 w-3.5 text-fg-subtle' />
              <input
                autoFocus
                value={query}
                onChange={(e) => setQuery(e.target.value)}
                placeholder='Filter…'
                className='w-full bg-transparent text-sm text-fg placeholder:text-fg-subtle focus:outline-none'
              />
            </div>
          )}
          <div className='scrollbar-thin max-h-60 overflow-y-auto'>
            {shown.map((opt) => {
              const active = opt.value === value;
              return (
                <button
                  key={opt.value}
                  type='button'
                  role='option'
                  aria-selected={active}
                  onClick={() => {
                    onChange(opt.value);
                    setQuery('');
                    setOpen(false);
                  }}
                  className='flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm hover:bg-surface-hover'
                >
                  <span
                    className={cn(
                      'grid h-3.5 w-3.5 shrink-0 place-items-center rounded-full border',
                      active ? 'border-primary' : 'border-fg-subtle',
                    )}
                  >
                    {active && <span className='h-1.5 w-1.5 rounded-full bg-primary' />}
                  </span>
                  <span className='truncate text-fg'>{labelOf(opt)}</span>
                  {opt.hint && <span className='truncate text-xs text-fg-subtle'>{opt.hint}</span>}
                  {opt.tone && (
                    <span
                      className={cn(
                        'ml-auto shrink-0 rounded px-1.5 py-0.5 font-mono text-xxs',
                        TONE_TAG[opt.tone],
                      )}
                    >
                      {opt.value}
                    </span>
                  )}
                </button>
              );
            })}
            {shown.length === 0 && (
              <p className='px-2.5 py-3 text-center text-xs text-fg-subtle'>No match</p>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

export function MultiSelectInput({ field, value, onChange, invalid, disabled }: ParamInputProps) {
  const options = field.options ?? [];
  const chosen = Array.isArray(value) ? value : [];
  const [open, setOpen] = useState(false);
  const ref = useDismiss(() => setOpen(false));

  function toggle(v: string) {
    onChange(chosen.includes(v) ? chosen.filter((x) => x !== v) : [...chosen, v]);
  }

  return (
    <div ref={ref} className='relative'>
      <div
        className={cn(
          'flex min-h-[2.5rem] flex-wrap items-center gap-1.5 rounded-md border bg-bg px-2 py-1.5',
          popoverRing(open, invalid, false),
          disabled && 'opacity-60',
        )}
      >
        {chosen.map((v) => (
          <span
            key={v}
            className='inline-flex items-center gap-1.5 rounded border border-border bg-surface-hover px-2 py-0.5 font-mono text-xs text-fg'
          >
            {v}
            {!disabled && (
              <button
                type='button'
                onClick={() => toggle(v)}
                className='text-fg-subtle hover:text-danger'
                aria-label={`Remove ${v}`}
              >
                <X className='h-3 w-3' />
              </button>
            )}
          </span>
        ))}
        <button
          type='button'
          disabled={disabled}
          onClick={() => setOpen((o) => !o)}
          className='inline-flex items-center gap-1 rounded border border-dashed border-border px-2 py-0.5 font-mono text-xs text-fg-muted hover:border-border-strong hover:text-fg disabled:cursor-not-allowed'
        >
          {chosen.length ? 'edit' : 'choose…'}
          <ChevronDown className={cn('h-3 w-3 transition-transform', open && 'rotate-180')} />
        </button>
      </div>
      {open && (
        <div className='scrollbar-thin absolute z-30 mt-1.5 max-h-60 w-full overflow-y-auto rounded-lg border border-border bg-bg-elevated p-1.5 shadow-popover'>
          {options.map((opt) => {
            const active = chosen.includes(opt.value);
            return (
              <button
                key={opt.value}
                type='button'
                onClick={() => toggle(opt.value)}
                className='flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm hover:bg-surface-hover'
              >
                <span
                  className={cn(
                    'grid h-3.5 w-3.5 shrink-0 place-items-center rounded border',
                    active ? 'border-primary bg-primary' : 'border-fg-subtle',
                  )}
                >
                  {active && <Check className='h-2.5 w-2.5 text-fg-onPrimary' />}
                </span>
                <span className='truncate text-fg'>{labelOf(opt)}</span>
                {opt.hint && <span className='truncate text-xs text-fg-subtle'>{opt.hint}</span>}
              </button>
            );
          })}
          {options.length === 0 && (
            <p className='px-2.5 py-3 text-center text-xs text-fg-subtle'>No options defined</p>
          )}
        </div>
      )}
    </div>
  );
}
