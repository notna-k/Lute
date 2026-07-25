/** One row of the run form: label, control, error/description, env hint. */
import { typeDef } from '../params/registry';
import { cn } from '@/lib/cn';
import type { ParamSpec, ParamValue } from '../params/types';

export interface RunFieldProps {
  spec: ParamSpec;
  value: ParamValue;
  onChange: (value: ParamValue) => void;
  error?: string;
  /** Renders the env-var hint. Off in layouts that show a dedicated env pane. */
  showEnv?: boolean;
  /** Compact rows for dense sidebars. */
  dense?: boolean;
  className?: string;
  onFocusCapture?: () => void;
}

export function RunField({
  spec,
  value,
  onChange,
  error,
  showEnv = true,
  dense,
  className,
  onFocusCapture,
}: RunFieldProps) {
  const { Input } = typeDef(spec.type);
  return (
    <div
      className={className}
      onFocusCapture={onFocusCapture}
      onMouseDown={onFocusCapture}
    >
      <div className={cn('flex items-baseline gap-2', dense ? 'mb-1.5' : 'mb-2')}>
        <span className={cn('font-semibold text-fg', dense ? 'text-xs' : 'text-sm')}>
          {spec.label}
        </span>
        {spec.required && (
          <span className='text-xxs font-medium uppercase tracking-wide text-warning'>
            required
          </span>
        )}
        {showEnv && (
          <span className='ml-auto font-mono text-xxs text-fg-subtle'>${spec.env}</span>
        )}
      </div>
      <Input spec={spec} value={value} onChange={onChange} invalid={Boolean(error)} />
      {error ? (
        <p className='mt-1.5 font-mono text-xs text-danger'>{error}</p>
      ) : (
        spec.description && (
          <p className={cn('mt-1.5 text-xs text-fg-muted', dense && 'text-xxs')}>
            {spec.description}
          </p>
        )
      )}
    </div>
  );
}
