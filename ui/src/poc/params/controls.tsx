/**
 * Runner-side controls: what a person filling in a build sees.
 *
 * Each control is dumb — value in, value out. The registry pairs them with
 * their config editor and validator.
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
  Upload,
  X,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import { TONE_TAG } from './tones';
import type { ParamInputProps, ParamOption } from './types';

const FIELD_BASE =
  'w-full rounded-md border bg-bg px-3 py-2 text-sm text-fg placeholder:text-fg-subtle transition-colors focus-visible:outline-none';

function fieldRing(invalid?: boolean) {
  return invalid
    ? 'border-danger focus-visible:ring-2 focus-visible:ring-danger/20'
    : 'border-border hover:border-border-strong focus-visible:border-primary focus-visible:ring-2 focus-visible:ring-primary/20';
}

function labelOf(o: ParamOption) {
  return o.label ?? o.value;
}

/** Closes a popover when a click lands outside it or Escape is pressed. */
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

// --- text ------------------------------------------------------------------

export function TextInput({ spec, value, onChange, invalid }: ParamInputProps) {
  const common = {
    value: String(value ?? ''),
    onChange: (e: { target: { value: string } }) => onChange(e.target.value),
    placeholder: spec.required ? 'required' : 'optional',
    className: cn(FIELD_BASE, fieldRing(invalid)),
  };
  if (spec.multiline) {
    return <textarea {...common} rows={4} className={cn(common.className, 'font-mono resize-y')} />;
  }
  return <input type='text' {...common} />;
}

// --- number ----------------------------------------------------------------

export function NumberInput({ spec, value, onChange, invalid }: ParamInputProps) {
  const step = spec.step ?? 1;
  const num = typeof value === 'number' ? value : Number(value);
  const bump = (delta: number) => {
    const next = (Number.isFinite(num) ? num : 0) + delta * step;
    const clamped = Math.min(spec.max ?? Infinity, Math.max(spec.min ?? -Infinity, next));
    // Steps like 0.1 accumulate float noise; round to the step's precision.
    const decimals = (String(step).split('.')[1] ?? '').length;
    onChange(Number(clamped.toFixed(decimals)));
  };
  return (
    <div className='flex items-stretch gap-2'>
      <div className='relative flex-1'>
        <input
          type='number'
          value={value === '' ? '' : String(value)}
          min={spec.min}
          max={spec.max}
          step={step}
          // An emptied number input stays empty rather than coercing to 0 — 0 is
          // a real value and coercion would defeat the `required` check.
          onChange={(e) => onChange(e.target.value === '' ? '' : Number(e.target.value))}
          className={cn(FIELD_BASE, fieldRing(invalid), 'font-mono pr-14')}
        />
        {spec.unit && (
          <span className='pointer-events-none absolute right-3 top-1/2 -translate-y-1/2 font-mono text-xxs text-fg-subtle'>
            {spec.unit}
          </span>
        )}
      </div>
      <div className='flex overflow-hidden rounded-md border border-border'>
        <button
          type='button'
          onClick={() => bump(-1)}
          className='px-2 text-fg-muted hover:bg-surface-hover'
          aria-label='Decrease'
        >
          −
        </button>
        <span className='w-px bg-border' />
        <button
          type='button'
          onClick={() => bump(1)}
          className='px-2 text-fg-muted hover:bg-surface-hover'
          aria-label='Increase'
        >
          +
        </button>
      </div>
    </div>
  );
}

// --- slider ----------------------------------------------------------------

export function SliderInput({ spec, value, onChange }: ParamInputProps) {
  const min = spec.min ?? 0;
  const max = spec.max ?? 100;
  const num = typeof value === 'number' ? value : Number(value) || min;
  const pct = ((num - min) / (max - min)) * 100;
  return (
    <div className='flex items-center gap-3'>
      <input
        type='range'
        min={min}
        max={max}
        step={spec.step ?? 1}
        value={num}
        onChange={(e) => onChange(Number(e.target.value))}
        className='h-1.5 flex-1 cursor-pointer appearance-none rounded-full bg-surface-hover accent-primary'
        style={{
          background: `linear-gradient(to right, rgb(var(--color-primary)) ${pct}%, rgb(var(--color-surface-hover)) ${pct}%)`,
        }}
      />
      <span className='min-w-[4rem] rounded-md border border-border bg-bg px-2 py-1 text-center font-mono text-xs text-fg'>
        {num}
        {spec.unit && <span className='text-fg-subtle'>{spec.unit}</span>}
      </span>
    </div>
  );
}

// --- bool ------------------------------------------------------------------

export function ToggleInput({ value, onChange }: ParamInputProps) {
  const on = Boolean(value);
  return (
    <button
      type='button'
      role='switch'
      aria-checked={on}
      onClick={() => onChange(!on)}
      className='inline-flex items-center gap-2.5'
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

export function SelectInput({ spec, value, onChange, invalid }: ParamInputProps) {
  const options = spec.options ?? [];
  const [open, setOpen] = useState(false);
  const [query, setQuery] = useState('');
  const ref = useDismiss(() => setOpen(false));
  const selected = options.find((o) => o.value === value);
  // Search only earns its place once the list is long enough to scan poorly.
  const searchable = options.length > 7;
  const shown = query
    ? options.filter((o) => `${o.value} ${labelOf(o)}`.toLowerCase().includes(query.toLowerCase()))
    : options;

  return (
    <div ref={ref} className='relative'>
      <button
        type='button'
        onClick={() => setOpen((o) => !o)}
        aria-haspopup='listbox'
        aria-expanded={open}
        className={cn(
          'flex w-full items-center gap-2.5 rounded-md border bg-bg px-3 py-2 text-left text-sm transition-colors',
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
        {selected?.hint && (
          <span className='truncate text-xs text-fg-muted'>· {selected.hint}</span>
        )}
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
          <div className='max-h-60 overflow-y-auto'>
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
                  {opt.hint && (
                    <span className='truncate text-xs text-fg-subtle'>{opt.hint}</span>
                  )}
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

export function MultiSelectInput({ spec, value, onChange, invalid }: ParamInputProps) {
  const options = spec.options ?? [];
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
          invalid ? 'border-danger' : open ? 'border-primary ring-2 ring-primary/20' : 'border-border'
        )}
      >
        {chosen.map((v) => (
          <span
            key={v}
            className='inline-flex items-center gap-1.5 rounded border border-border bg-surface-hover px-2 py-0.5 font-mono text-xs text-fg'
          >
            {v}
            <button
              type='button'
              onClick={() => toggle(v)}
              className='text-fg-subtle hover:text-danger'
              aria-label={`Remove ${v}`}
            >
              <X className='h-3 w-3' />
            </button>
          </span>
        ))}
        <button
          type='button'
          onClick={() => setOpen((o) => !o)}
          className='inline-flex items-center gap-1 rounded border border-dashed border-border px-2 py-0.5 font-mono text-xs text-fg-muted hover:border-border-strong hover:text-fg'
        >
          {chosen.length ? 'edit' : 'choose…'}
          <ChevronDown className={cn('h-3 w-3 transition-transform', open && 'rotate-180')} />
        </button>
        {(spec.minSelected || spec.maxSelected) && (
          <span className='ml-auto pr-1 font-mono text-xxs text-fg-subtle'>
            {chosen.length}
            {spec.maxSelected ? `/${spec.maxSelected}` : ''}
          </span>
        )}
      </div>
      {open && (
        <div className='absolute z-30 mt-1.5 max-h-60 w-full overflow-y-auto rounded-lg border border-border bg-bg-elevated p-1.5 shadow-popover'>
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

function Calendar30({ value, onPick }: { value: string; onPick: (iso: string) => void }) {
  const selected = value ? new Date(`${value}T00:00:00`) : null;
  const [view, setView] = useState(() => selected ?? new Date());
  const today = toISODate(new Date());

  const cells = useMemo(() => {
    const first = new Date(view.getFullYear(), view.getMonth(), 1);
    const offset = (first.getDay() + 6) % 7; // Monday-first
    const start = new Date(view.getFullYear(), view.getMonth(), 1 - offset);
    return Array.from({ length: 42 }, (_, i) =>
      new Date(start.getFullYear(), start.getMonth(), start.getDate() + i)
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

/** Quick relative picks — the fast path for the common cases. */
const DATE_SHORTCUTS: { label: string; days: number }[] = [
  { label: 'Today', days: 0 },
  { label: 'Tomorrow', days: 1 },
  { label: '+1 week', days: 7 },
];

export function DateInput({ value, onChange, invalid }: ParamInputProps) {
  const [open, setOpen] = useState(false);
  const ref = useDismiss(() => setOpen(false));
  const str = String(value ?? '');

  return (
    <div ref={ref} className='relative'>
      <div className='flex items-center gap-2'>
        <button
          type='button'
          onClick={() => setOpen((o) => !o)}
          className={cn(
            'flex flex-1 items-center gap-2.5 rounded-md border bg-bg px-3 py-2 text-left font-mono text-sm transition-colors',
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
        {DATE_SHORTCUTS.map((s) => (
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
          <Calendar30
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

export function DateTimeInput({ value, onChange, invalid }: ParamInputProps) {
  const [open, setOpen] = useState(false);
  const ref = useDismiss(() => setOpen(false));
  const str = String(value ?? '');
  const [datePart = '', timePart = '09:00'] = str ? str.split('T') : [];
  const tz = Intl.DateTimeFormat().resolvedOptions().timeZone;

  const set = (d: string, t: string) => onChange(d ? `${d}T${t}` : '');

  return (
    <div ref={ref} className='relative'>
      <div className='flex items-center gap-2'>
        <button
          type='button'
          onClick={() => setOpen((o) => !o)}
          className={cn(
            'flex flex-1 items-center gap-2.5 rounded-md border bg-bg px-3 py-2 text-left font-mono text-sm transition-colors',
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
            disabled={!datePart}
            onChange={(e) => set(datePart, e.target.value)}
            className='bg-transparent font-mono text-sm text-fg focus:outline-none'
          />
        </div>
        <span className='font-mono text-xxs text-fg-subtle'>{tz}</span>
      </div>
      {open && (
        <div className='absolute z-30 mt-1.5 rounded-lg border border-border bg-bg-elevated shadow-popover'>
          <Calendar30
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

export function SecretInput({ spec }: ParamInputProps) {
  return (
    <div className='flex items-center gap-2.5 rounded-md border border-dashed border-border bg-bg px-3 py-2'>
      <Lock className='h-4 w-4 text-info' />
      <code className='font-mono tracking-[0.2em] text-fg-muted'>••••••••••••</code>
      <span className='ml-auto font-mono text-xxs text-fg-subtle'>
        {spec.secretRef ? `from ${spec.secretRef}` : 'resolved at run · never logged'}
      </span>
    </div>
  );
}

// --- file ------------------------------------------------------------------

export function FileInput({ spec, value, onChange, invalid }: ParamInputProps) {
  const name = String(value ?? '');
  return (
    <div
      className={cn(
        'flex items-center gap-3 rounded-md border border-dashed px-3 py-3',
        invalid ? 'border-danger' : 'border-border hover:border-border-strong'
      )}
    >
      <Upload className='h-4 w-4 text-fg-muted' />
      {name ? (
        <>
          <code className='truncate font-mono text-xs text-fg'>{name}</code>
          <button
            type='button'
            onClick={() => onChange('')}
            className='ml-auto text-fg-subtle hover:text-danger'
            aria-label='Remove file'
          >
            <X className='h-3.5 w-3.5' />
          </button>
        </>
      ) : (
        <>
          <span className='text-sm text-fg-subtle'>Drop a file or</span>
          {/* PoC: no real upload — a click stands in for the object-store round trip. */}
          <button
            type='button'
            onClick={() => onChange('release-notes.md')}
            className='rounded-md border border-border px-2 py-1 text-xs text-fg-muted hover:text-fg'
          >
            browse
          </button>
          <span className='ml-auto font-mono text-xxs text-fg-subtle'>{spec.accept ?? 'any'}</span>
        </>
      )}
    </div>
  );
}
