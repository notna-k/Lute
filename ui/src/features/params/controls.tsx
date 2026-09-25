// Runner-side controls: value in, value out. The registry pairs each with its config editor and validator.
import { Lock } from 'lucide-react';
import { cn } from '@/lib/cn';
import { FIELD_BASE, ring } from './controlStyles';
import type { ParamInputProps } from './types';

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

export function NumberInput({ value, onChange, invalid, disabled }: ParamInputProps) {
  return (
    <input
      type='number'
      value={value === '' || value === undefined ? '' : String(value)}
      disabled={disabled}
      // Stays empty rather than coercing to 0, which would defeat the `required` check.
      onChange={(e) => onChange(e.target.value === '' ? '' : Number(e.target.value))}
      className={cn(FIELD_BASE, ring(invalid), 'font-mono')}
    />
  );
}

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
          on ? 'border-success bg-success-subtle' : 'border-border bg-surface-hover',
        )}
      >
        <span
          className={cn(
            'absolute top-0.5 h-4 w-4 rounded-full transition-all',
            on ? 'left-[1.375rem] bg-success' : 'left-0.5 bg-fg-subtle',
          )}
        />
      </span>
      <span className='font-mono text-xs text-fg-muted'>{on ? 'true' : 'false'}</span>
    </button>
  );
}

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
