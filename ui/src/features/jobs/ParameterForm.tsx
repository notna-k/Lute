import {
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import {
  Calendar,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Lock,
  Plus,
  X,
} from 'lucide-react';
import { cn } from '@/lib/cn';
import type {
  ParameterField,
  ParameterOption,
  ParameterValue,
  ParameterValues,
} from '@/types/jobs';

const TONE_TAG: Record<string, string> = {
  neutral: 'text-fg-muted bg-surface-hover',
  success: 'text-success-fg bg-success-subtle',
  warning: 'text-warning-fg bg-warning-subtle',
  danger: 'text-danger-fg bg-danger-subtle',
};

function defaultsOf(fields: ParameterField[]): ParameterValues {
  const values: ParameterValues = {};
  for (const f of fields) {
    if (f.default !== undefined) {
      values[f.name] = f.default;
    } else if (f.type === 'multiselect') {
      values[f.name] = [];
    } else if (f.type === 'bool') {
      values[f.name] = false;
    } else {
      values[f.name] = '';
    }
  }
  return values;
}

/** Closes a popover when a click lands outside its container. */
function useOutsideClick(onClose: () => void) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function handle(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    document.addEventListener('mousedown', handle);
    return () => document.removeEventListener('mousedown', handle);
  }, [onClose]);
  return ref;
}

interface FieldRowProps {
  field: ParameterField;
  children: ReactNode;
}

function FieldRow({ field, children }: FieldRowProps) {
  return (
    <div className='border-b border-dashed border-border py-4 last:border-b-0'>
      <div className='mb-2.5 flex items-baseline gap-2'>
        <span className='text-sm font-semibold text-fg'>{field.label}</span>
        {field.required && (
          <span className='text-xxs font-medium uppercase tracking-wide text-warning'>
            required
          </span>
        )}
        <span className='ml-auto font-mono text-xxs text-fg-subtle'>
          ${field.envVar}
        </span>
      </div>
      {children}
      {field.description && (
        <p className='mt-2 text-xs text-fg-muted'>{field.description}</p>
      )}
    </div>
  );
}

interface SelectProps {
  options: ParameterOption[];
  value: string;
  onChange: (value: string) => void;
}

function ConsoleSelect({ options, value, onChange }: SelectProps) {
  const [open, setOpen] = useState(false);
  const ref = useOutsideClick(() => setOpen(false));
  const selected = options.find((o) => o.value === value) ?? options[0];

  return (
    <div ref={ref} className='relative'>
      <button
        type='button'
        onClick={() => setOpen((o) => !o)}
        className={cn(
          'flex w-full items-center gap-2.5 rounded-md border bg-bg px-3 py-2.5 text-left text-sm transition-colors',
          open ? 'border-primary ring-2 ring-primary/20' : 'border-border hover:border-border-strong'
        )}
        aria-haspopup='listbox'
        aria-expanded={open}
      >
        {selected?.tone && (
          <span
            className={cn(
              'rounded px-1.5 py-0.5 font-mono text-xxs',
              TONE_TAG[selected.tone]
            )}
          >
            {selected.value}
          </span>
        )}
        <span className='text-fg'>{selected?.label}</span>
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
          className='absolute z-20 mt-1.5 w-full rounded-lg border border-border bg-bg-elevated p-1.5 shadow-popover'
          role='listbox'
        >
          {options.map((opt) => {
            const active = opt.value === value;
            return (
              <button
                key={opt.value}
                type='button'
                role='option'
                aria-selected={active}
                onClick={() => {
                  onChange(opt.value);
                  setOpen(false);
                }}
                className='flex w-full items-center gap-2.5 rounded-md px-2.5 py-2 text-left text-sm hover:bg-surface-hover'
              >
                <span
                  className={cn(
                    'grid h-3.5 w-3.5 place-items-center rounded-full border',
                    active ? 'border-primary' : 'border-fg-subtle'
                  )}
                >
                  {active && <span className='h-1.5 w-1.5 rounded-full bg-primary' />}
                </span>
                <span className='text-fg'>{opt.label}</span>
                {opt.hint && (
                  <span className='truncate text-xs text-fg-subtle'>{opt.hint}</span>
                )}
                {opt.tone && (
                  <span
                    className={cn(
                      'ml-auto rounded px-1.5 py-0.5 font-mono text-xxs',
                      TONE_TAG[opt.tone]
                    )}
                  >
                    {opt.value}
                  </span>
                )}
              </button>
            );
          })}
        </div>
      )}
    </div>
  );
}

interface MultiSelectProps {
  options: ParameterOption[];
  value: string[];
  onChange: (value: string[]) => void;
}

function ConsoleMultiSelect({ options, value, onChange }: MultiSelectProps) {
  const [open, setOpen] = useState(false);
  const ref = useOutsideClick(() => setOpen(false));
  const available = options.filter((o) => !value.includes(o.value));

  return (
    <div ref={ref} className='relative'>
      <div className='flex min-h-[2.75rem] flex-wrap items-center gap-2 rounded-md border border-border bg-bg px-2.5 py-2'>
        {value.map((v) => (
          <span
            key={v}
            className='inline-flex items-center gap-1.5 rounded-md border border-border bg-surface-hover px-2 py-1 font-mono text-xs text-fg'
          >
            {v}
            <button
              type='button'
              onClick={() => onChange(value.filter((x) => x !== v))}
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
          disabled={available.length === 0}
          className='inline-flex items-center gap-1 rounded-md border border-dashed border-border px-2 py-1 font-mono text-xs text-fg-muted hover:border-border-strong disabled:opacity-40'
        >
          <Plus className='h-3 w-3' /> add
        </button>
      </div>
      {open && available.length > 0 && (
        <div className='absolute z-20 mt-1.5 w-full rounded-lg border border-border bg-bg-elevated p-1.5 shadow-popover'>
          {available.map((opt) => (
            <button
              key={opt.value}
              type='button'
              onClick={() => {
                onChange([...value, opt.value]);
                setOpen(false);
              }}
              className='flex w-full items-center gap-2 rounded-md px-2.5 py-2 text-left font-mono text-sm text-fg hover:bg-surface-hover'
            >
              <Plus className='h-3.5 w-3.5 text-fg-subtle' />
              {opt.label}
            </button>
          ))}
        </div>
      )}
    </div>
  );
}

const WEEKDAYS = ['M', 'T', 'W', 'T', 'F', 'S', 'S'];
const MONTHS = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];

function toISODate(d: Date): string {
  return d.toISOString().slice(0, 10);
}

interface DateProps {
  value: string;
  onChange: (value: string) => void;
}

function ConsoleDate({ value, onChange }: DateProps) {
  const selected = value ? new Date(`${value}T00:00:00`) : null;
  const [view, setView] = useState(() => selected ?? new Date());

  const weeks = useMemo(() => {
    const year = view.getFullYear();
    const month = view.getMonth();
    const first = new Date(year, month, 1);
    // Monday-first offset.
    const offset = (first.getDay() + 6) % 7;
    const start = new Date(year, month, 1 - offset);
    const cells: Date[] = [];
    for (let i = 0; i < 42; i += 1) {
      cells.push(new Date(start.getFullYear(), start.getMonth(), start.getDate() + i));
    }
    const rows: Date[][] = [];
    for (let i = 0; i < 6; i += 1) rows.push(cells.slice(i * 7, i * 7 + 7));
    return rows;
  }, [view]);

  function shiftMonth(delta: number) {
    setView((v) => new Date(v.getFullYear(), v.getMonth() + delta, 1));
  }

  return (
    <div className='flex flex-wrap items-start gap-3'>
      <div className='flex min-w-[9rem] flex-1 items-center gap-2.5 rounded-md border border-border bg-bg px-3 py-2.5 font-mono text-sm text-fg'>
        <Calendar className='h-4 w-4 text-primary' />
        {value || 'not set'}
      </div>
      <div className='w-[15rem] rounded-lg border border-border bg-bg-elevated p-3'>
        <div className='mb-2 flex items-center justify-between'>
          <button
            type='button'
            onClick={() => shiftMonth(-1)}
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
            onClick={() => shiftMonth(1)}
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
          {weeks.flat().map((day) => {
            const iso = toISODate(day);
            const inMonth = day.getMonth() === view.getMonth();
            const isSelected = value === iso;
            return (
              <button
                key={iso}
                type='button'
                onClick={() => onChange(iso)}
                className={cn(
                  'rounded py-1.5 font-mono text-xs transition-colors',
                  isSelected
                    ? 'bg-primary font-bold text-fg-onPrimary'
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
    </div>
  );
}

interface ToggleProps {
  value: boolean;
  onChange: (value: boolean) => void;
}

function ConsoleToggle({ value, onChange }: ToggleProps) {
  return (
    <button
      type='button'
      role='switch'
      aria-checked={value}
      onClick={() => onChange(!value)}
      className='inline-flex items-center gap-2.5'
    >
      <span
        className={cn(
          'relative h-6 w-11 rounded-full border transition-colors',
          value ? 'border-success bg-success-subtle' : 'border-border bg-surface-hover'
        )}
      >
        <span
          className={cn(
            'absolute top-0.5 h-4 w-4 rounded-full transition-all',
            value ? 'left-[1.375rem] bg-success' : 'left-0.5 bg-fg-subtle'
          )}
        />
      </span>
      <span className='font-mono text-xs text-fg-muted'>{value ? 'true' : 'false'}</span>
    </button>
  );
}

function ConsoleSecret({ secretRef }: { secretRef?: string }) {
  return (
    <div className='flex items-center gap-2.5 rounded-md border border-dashed border-border bg-bg px-3 py-2.5'>
      <Lock className='h-4 w-4 text-info' />
      <code className='font-mono tracking-[0.2em] text-fg-muted'>••••••••••••</code>
      <span className='ml-auto font-mono text-xxs text-fg-subtle'>
        {secretRef ? `from ${secretRef}` : 'resolved at run · never logged'}
      </span>
    </div>
  );
}

export interface ParameterFormProps {
  fields: ParameterField[];
  onChange?: (values: ParameterValues) => void;
}

export function ParameterForm({ fields, onChange }: ParameterFormProps) {
  const [values, setValues] = useState<ParameterValues>(() => defaultsOf(fields));

  function set(name: string, value: ParameterValue) {
    setValues((prev) => {
      const next = { ...prev, [name]: value };
      onChange?.(next);
      return next;
    });
  }

  return (
    <div>
      {fields.map((field) => (
        <FieldRow key={field.name} field={field}>
          {field.type === 'select' && (
            <ConsoleSelect
              options={field.options ?? []}
              value={String(values[field.name] ?? '')}
              onChange={(v) => set(field.name, v)}
            />
          )}
          {field.type === 'multiselect' && (
            <ConsoleMultiSelect
              options={field.options ?? []}
              value={(values[field.name] as string[]) ?? []}
              onChange={(v) => set(field.name, v)}
            />
          )}
          {field.type === 'date' && (
            <ConsoleDate
              value={String(values[field.name] ?? '')}
              onChange={(v) => set(field.name, v)}
            />
          )}
          {(field.type === 'datetime') && (
            <ConsoleDate
              value={String(values[field.name] ?? '').slice(0, 10)}
              onChange={(v) => set(field.name, v)}
            />
          )}
          {field.type === 'bool' && (
            <ConsoleToggle
              value={Boolean(values[field.name])}
              onChange={(v) => set(field.name, v)}
            />
          )}
          {field.type === 'secret' && <ConsoleSecret secretRef={field.secretRef} />}
          {(field.type === 'string' || field.type === 'number') && (
            <input
              type={field.type === 'number' ? 'number' : 'text'}
              value={String(values[field.name] ?? '')}
              onChange={(e) =>
                set(
                  field.name,
                  field.type === 'number' ? Number(e.target.value) : e.target.value
                )
              }
              className='w-full rounded-md border border-border bg-bg px-3 py-2.5 text-sm text-fg placeholder:text-fg-subtle focus-visible:border-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20'
              placeholder={field.required ? 'required' : 'optional'}
            />
          )}
        </FieldRow>
      ))}
    </div>
  );
}
