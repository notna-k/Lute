import { useMemo, useState } from 'react';
import { Calendar, ChevronLeft, ChevronRight, Clock } from 'lucide-react';
import { cn } from '@/lib/cn';
import { popoverRing, useDismiss } from './controlStyles';
import type { ParamInputProps } from './types';

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
      (_, i) => new Date(start.getFullYear(), start.getMonth(), start.getDate() + i),
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
                      : 'text-fg-subtle/50 hover:bg-surface-hover',
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
            popoverRing(open, invalid),
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
            popoverRing(open, invalid),
          )}
        >
          <Calendar className='h-4 w-4 text-primary' />
          <span className={datePart ? 'text-fg' : 'text-fg-subtle'}>{datePart || 'not set'}</span>
        </button>
        <div
          className={cn(
            'flex items-center gap-2 rounded-md border border-border bg-bg px-3 py-2',
            !datePart && 'opacity-50',
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
