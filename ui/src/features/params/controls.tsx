/**
 * Runner-side controls: what someone filling in a build sees.
 *
 * Each control is dumb — value in, value out. The registry pairs it with its
 * config editor and validator.
 */
import { useEffect, useMemo, useRef, useState } from 'react';
import {
  Calendar,
  Check,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Clock,
  Lock,
  Search,
  X,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { TONE_TAG } from './tones';
import type { ParamInputProps } from './types';
import type { ParameterOption } from '@/types/jobs';

const FIELD_BASE =
  'w-full rounded-md border bg-bg px-3 py-2 text-sm text-fg placeholder:text-fg-subtle transition-colors focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60';

function ring(invalid?: boolean) {
  return invalid
    ? 'border-danger focus-visible:ring-2 focus-visible:ring-danger/20'
    : 'border-border hover:border-border-strong focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-primary/20';
}

function labelOf(o: ParameterOption) {
  return o.label || o.value;
}

/** Closes a popover on an outside click or Escape. */
function useDismiss(onClose: () => void) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [onClose]);
  return ref;
}

// --- string ----------------------------------------------------------------

export function TextInput({ field, value, onChange, invalid, disabled }: ParamInputProps) {
  return (
    <input
      type='text'
      value={String(value ?? '')}
      disabled={disabled}
      onChange={(e) => onChange(e.target.value)}
      placeholder={field.required ? 'required' : 'optional'}
      className={cn(FIELD_BASE, ring(invalid))}
    />
  );
}

// --- number ----------------------------------------------------------------

export function NumberInput({ value, onChange, invalid, disabled }: ParamInputProps) {
  return (
    <input
      type='number'
      value={value === '' || value === undefined ? '' : String(value)}
      disabled={disabled}
      // An emptied number input stays empty rather than coercing to 0 — 0 is a
      // real value and coercing would defeat the `required` check.
      onChange={(e) => onChange(e.target.value === '' ? '' : Number(e.target.value))}
      className={cn(FIELD_BASE, ring(invalid), 'font-mono')}
    />
  );
}

// --- bool ------------------------------------------------------------------

export function ToggleInput({ value, onChange, disabled }: ParamInputProps) {
  const on = Boolean(value);
  return (
    <button
      type='button'
      role='switch'
      aria-checked={on}
      disabled={disabled}
      onClick={() => onChange(!on)}
      className='inline-flex items-center gap-2.5 disabled:opacity-60'
    >
      <span
        className={cn(
          'relative h-6 w-11 rounded-full border transition-colors',
          on ? 'border-success bg-success-subtle' : 'border-border bg-surface-hover'
        )}
      >
        <span
          className={cn(
            'absolute top-0.5 h-4 w-4 rounded-full transition-all',
            on ? 'left-[1.375rem] bg-success' : 'left-0.5 bg-fg-subtle'
          )}
        />
      </span>
      <span className='font-mono text-xs text-fg-muted'>{on ? 'true' : 'false'}</span>
    </button>
  );
}

// --- select ----------------------------------------------------------------

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
          invalid
            ? 'border-danger'
            : open
              ? 'border-primary ring-2 ring-primary/20'
              : 'border-border hover:border-border-strong'
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
        {selected?.hint && <span className='truncate text-xs text-fg-muted'>· {selected.hint}</span>}
        <ChevronDown
          className={cn(
            'ml-auto h-4 w-4 shrink-0 text-fg-muted transition-transform',
            open && 'rotate-180'
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
                      active ? 'border-primary' : 'border-fg-subtle'
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
                        TONE_TAG[opt.tone]
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

// --- multiselect -----------------------------------------------------------

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
          invalid ? 'border-danger' : open ? 'border-primary ring-2 ring-primary/20' : 'border-border',
          disabled && 'opacity-60'
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
                    active ? 'border-primary bg-primary' : 'border-fg-subtle'
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

// --- date / datetime -------------------------------------------------------

const WEEKDAYS = ['M', 'T', 'W', 'T', 'F', 'S', 'S'];
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

/**
 * Formats a Date as YYYY-MM-DD in LOCAL time. `toISOString()` converts to UTC
 * first and shifts the day by one for any timezone east of UTC — the picker
 * would then submit yesterday's date.
 */
function toISODate(d: Date): string {
  const month = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${d.getFullYear()}-${month}-${day}`;
}

function MonthGrid({ value, onPick }: { value: string; onPick: (iso: string) => void }) {
  const selected = value ? new Date(`${value}T00:00:00`) : null;
  const [view, setView] = useState(() => selected ?? new Date());
  const today = toISODate(new Date());

  const cells = useMemo(() => {
    const first = new Date(view.getFullYear(), view.getMonth(), 1);
    const offset = (first.getDay() + 6) % 7; // Monday-first
    const start = new Date(view.getFullYear(), view.getMonth(), 1 - offset);
    return Array.from(
      { length: 42 },
      (_, i) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + i)
    );
  }, [view]);

  return (
    <div className='w-[15.5rem] p-3'>
      <div className='mb-2 flex items-center justify-between'>
        <button
          type='button'
          onClick={() => setView((v) => new Date(v.getFullYear(), v.getMonth() - 1, 1))}
          className='rounded p-1 text-fg-muted hover:bg-surface-hover'
          aria-label='Previous month'
        >
          <ChevronLeft className='h-4 w-4' />
        </button>
        <span className='font-mono text-xs text-fg'>
          {MONTHS[view.getMonth()]} {view.getFullYear()}
        </span>
        <button
          type='button'
          onClick={() => setView((v) => new Date(v.getFullYear(), v.getMonth() + 1, 1))}
          className='rounded p-1 text-fg-muted hover:bg-surface-hover'
          aria-label='Next month'
        >
          <ChevronRight className='h-4 w-4' />
        </button>
      </div>
      <div className='grid grid-cols-7 gap-0.5 text-center'>
        {WEEKDAYS.map((d, i) => (
          <span key={i} className='py-1 font-mono text-xxs text-fg-subtle'>
            {d}
          </span>
        ))}
        {cells.map((day) => {
          const iso = toISODate(day);
          const inMonth = day.getMonth() === view.getMonth();
          return (
            <button
              key={iso}
              type='button'
              onClick={() => onPick(iso)}
              className={cn(
                'rounded py-1.5 font-mono text-xs transition-colors',
                value === iso
                  ? 'bg-primary font-bold text-fg-onPrimary'
                  : iso === today
                    ? 'text-primary ring-1 ring-inset ring-primary/40 hover:bg-surface-hover'
                    : inMonth
                      ? 'text-fg-muted hover:bg-surface-hover'
                      : 'text-fg-subtle/50 hover:bg-surface-hover'
              )}
            >
              {day.getDate()}
            </button>
          );
        })}
      </div>
    </div>
  );
}

/** Relative picks — the fast path for the common cases. */
const SHORTCUTS: { label: string; days: number }[] = [
  { label: 'Today', days: 0 },
  { label: 'Tomorrow', days: 1 },
  { label: '+1w', days: 7 },
];

export function DateInput({ value, onChange, invalid, disabled }: ParamInputProps) {
  const [open, setOpen] = useState(false);
  const ref = useDismiss(() => setOpen(false));
  const str = String(value ?? '');

  return (
    <div ref={ref} className='relative'>
      <div className='flex flex-wrap items-center gap-2'>
        <button
          type='button'
          disabled={disabled}
          onClick={() => setOpen((o) => !o)}
          className={cn(
            'flex min-w-[10rem] flex-1 items-center gap-2.5 rounded-md border bg-bg px-3 py-2 text-left font-mono text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-60',
            invalid
              ? 'border-danger'
              : open
                ? 'border-primary ring-2 ring-primary/20'
                : 'border-border hover:border-border-strong'
          )}
        >
          <Calendar className='h-4 w-4 text-primary' />
          <span className={str ? 'text-fg' : 'text-fg-subtle'}>{str || 'not set'}</span>
        </button>
        {!disabled &&
          SHORTCUTS.map((s) => (
            <button
              key={s.label}
              type='button'
              onClick={() => {
                const d = new Date();
                d.setDate(d.getDate() + s.days);
                onChange(toISODate(d));
              }}
              className='rounded-md border border-border px-2 py-1.5 text-xs text-fg-muted hover:border-border-strong hover:text-fg'
            >
              {s.label}
            </button>
          ))}
      </div>
      {open && (
        <div className='absolute z-30 mt-1.5 rounded-lg border border-border bg-bg-elevated shadow-popover'>
          <MonthGrid
            value={str}
            onPick={(iso) => {
              onChange(iso);
              setOpen(false);
            }}
          />
        </div>
      )}
    </div>
  );
}

export function DateTimeInput({ value, onChange, invalid, disabled }: ParamInputProps) {
  const [open, setOpen] = useState(false);
  const ref = useDismiss(() => setOpen(false));
  const str = String(value ?? '');
  const [datePart = '', rest = ''] = str ? str.split('T') : [];
  const timePart = rest.slice(0, 5) || '09:00';
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;

  // The server parses RFC-3339, so keep the offset on the wire.
  const set = (d: string, t: string) => {
    if (!d) return onChange('');
    const local = new Date(`${d}T${t}:00`);
    const off = -local.getTimezoneOffset();
    const sign = off >= 0 ? '+' : '-';
    const hh = String(Math.floor(Math.abs(off) / 60)).padStart(2, '0');
    const mm = String(Math.abs(off) % 60).padStart(2, '0');
    onChange(`${d}T${t}:00${sign}${hh}:${mm}`);
  };

  return (
    <div ref={ref} className='relative'>
      <div className='flex flex-wrap items-center gap-2'>
        <button
          type='button'
          disabled={disabled}
          onClick={() => setOpen((o) => !o)}
          className={cn(
            'flex min-w-[9rem] flex-1 items-center gap-2.5 rounded-md border bg-bg px-3 py-2 text-left font-mono text-sm transition-colors disabled:cursor-not-allowed disabled:opacity-60',
            invalid
              ? 'border-danger'
              : open
                ? 'border-primary ring-2 ring-primary/20'
                : 'border-border hover:border-border-strong'
          )}
        >
          <Calendar className='h-4 w-4 text-primary' />
          <span className={datePart ? 'text-fg' : 'text-fg-subtle'}>{datePart || 'not set'}</span>
        </button>
        <div
          className={cn(
            'flex items-center gap-2 rounded-md border border-border bg-bg px-3 py-2',
            !datePart && 'opacity-50'
          )}
        >
          <Clock className='h-4 w-4 text-fg-muted' />
          <input
            type='time'
            value={timePart}
            disabled={!datePart || disabled}
            onChange={(e) => set(datePart, e.target.value)}
            className='bg-transparent font-mono text-sm text-fg focus:outline-none'
          />
        </div>
        <span className='font-mono text-xxs text-fg-subtle'>{tz}</span>
      </div>
      {open && (
        <div className='absolute z-30 mt-1.5 rounded-lg border border-border bg-bg-elevated shadow-popover'>
          <MonthGrid
            value={datePart}
            onPick={(iso) => {
              set(iso, timePart);
              setOpen(false);
            }}
          />
        </div>
      )}
    </div>
  );
}

// --- secret ----------------------------------------------------------------

export function SecretInput({ field }: ParamInputProps) {
  return (
    <div className='flex items-center gap-2.5 rounded-md border border-dashed border-border bg-bg px-3 py-2'>
      <Lock className='h-4 w-4 text-info' />
      <code className='font-mono tracking-[0.2em] text-fg-muted'>••••••••••••</code>
      <span className='ml-auto truncate font-mono text-xxs text-fg-subtle'>
        {field.secretRef ? `from ${field.secretRef}` : 'resolved at run · never logged'}
      </span>
    </div>
  );
}
