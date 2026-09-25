/** One row of a run form: label, control, error or help text, env hint. */
import { typeDef } from './registry';
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
          <span className='text-xxs font-medium uppercase tracking-wide text-warning'>
            required
          </span>
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
