/** One row of a run form: label, control, error or help text, env hint. */
import { typeDef } from './registry';
import { cn } from '@/lib/cn';
import type { ParameterField, ParameterValue } from '@/types/jobs';

export interface ParamFieldProps {
  field: ParameterField;
  value: ParameterValue;
  onChange: (value: ParameterValue) => void;
  error?: string;
  /** Hides the `$ENV_VAR` hint in layouts that show a dedicated env preview. */
  showEnv?: boolean;
  disabled?: boolean;
  className?: string;
  onFocusCapture?: () => void;
}

export function ParamField({
  field,
  value,
  onChange,
  error,
  showEnv = true,
  disabled,
  className,
  onFocusCapture,
}: ParamFieldProps) {
  const { Input } = typeDef(field.type);
  return (
    <div className={className} onFocusCapture={onFocusCapture}>
      <div className='mb-2 flex items-baseline gap-2'>
        <span className='text-sm font-semibold text-fg'>{field.label || field.name}</span>
        {field.required && (
          <span className='text-xxs font-medium uppercase tracking-wide text-warning'>required</span>
        )}
        {showEnv && (
          <span className='ml-auto truncate font-mono text-xxs text-fg-subtle'>
            ${field.envVar}
          </span>
        )}
      </div>
      <Input
        field={field}
        value={value}
        onChange={onChange}
        invalid={Boolean(error)}
        disabled={disabled}
      />
      {error ? (
        <p className='mt-1.5 font-mono text-xs text-danger'>{error}</p>
      ) : (
        field.description && <p className='mt-1.5 text-xs text-fg-muted'>{field.description}</p>
      )}
    </div>
  );
}

/** Compact, non-interactive summary of a field — used in read-only schema views. */
export function ParamSummary({ field }: { field: ParameterField }) {
  const def = typeDef(field.type);
  const Icon = def.icon;
  return (
    <div className='flex items-center gap-2 py-1'>
      <Icon className='h-3.5 w-3.5 shrink-0 text-fg-subtle' />
      <span className='truncate text-sm text-fg'>{field.label || field.name}</span>
      <span className='font-mono text-xxs text-fg-subtle'>{field.type}</span>
      {field.required && <span className={cn('text-xxs text-warning')}>●</span>}
      <span className='ml-auto truncate font-mono text-xxs text-fg-subtle'>${field.envVar}</span>
    </div>
  );
}
