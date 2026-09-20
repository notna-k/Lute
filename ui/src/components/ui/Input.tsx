import {
  forwardRef,
  type InputHTMLAttributes,
  type TextareaHTMLAttributes,
  type SelectHTMLAttributes,
  type ReactNode,
} from 'react';
import { cn } from '@/lib/cn';

/**
 * Text entry is monospaced throughout the panel: nearly every value typed here
 * is an identifier, a tag, a path or a cron expression, and a proportional font
 * makes those harder to compare character by character.
 */
const BASE_FIELD =
  'w-full border border-border bg-surface px-2 font-mono text-[12.5px] text-fg outline-none transition-colors focus:border-border-strong disabled:cursor-not-allowed disabled:opacity-50';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  leftIcon?: ReactNode;
  rightIcon?: ReactNode;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, leftIcon, rightIcon, ...props },
  ref,
) {
  if (leftIcon || rightIcon) {
    return (
      <div className='relative w-full'>
        {leftIcon && (
          <span className='pointer-events-none absolute left-2.5 top-1/2 -translate-y-1/2 text-fg-subtle'>
            {leftIcon}
          </span>
        )}
        <input
          ref={ref}
          className={cn(BASE_FIELD, 'h-[30px]', leftIcon && 'pl-8', rightIcon && 'pr-8', className)}
          {...props}
        />
        {rightIcon && (
          <span className='absolute right-2.5 top-1/2 -translate-y-1/2 text-fg-subtle'>
            {rightIcon}
          </span>
        )}
      </div>
    );
  }
  return <input ref={ref} className={cn(BASE_FIELD, 'h-[30px]', className)} {...props} />;
});

export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ className, ...props }, ref) {
  return (
    <textarea
      ref={ref}
      className={cn(BASE_FIELD, 'min-h-[80px] py-2 leading-[1.65]', className)}
      style={{ tabSize: 2 }}
      {...props}
    />
  );
});

export interface NativeSelectProps extends SelectHTMLAttributes<HTMLSelectElement> {
  placeholder?: string;
}

export const NativeSelect = forwardRef<HTMLSelectElement, NativeSelectProps>(function NativeSelect(
  { className, children, ...props },
  ref,
) {
  return (
    <select
      ref={ref}
      // The chevron is drawn with currentColor so it follows the theme
      // without a second asset per palette.
      className={cn(
        BASE_FIELD,
        'h-[30px] appearance-none bg-no-repeat pr-7 [background-position:right_8px_center] [background-size:14px]',
        className,
      )}
      style={{
        backgroundImage:
          "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='14' height='14' fill='none' stroke='currentColor' stroke-width='1.75' stroke-linecap='round' stroke-linejoin='round' viewBox='0 0 24 24'%3E%3Cpath d='m6 9 6 6 6-6'/%3E%3C/svg%3E\")",
      }}
      {...props}
    >
      {children}
    </select>
  );
});

export interface FieldProps {
  label?: ReactNode;
  htmlFor?: string;
  /** Environment variable or other technical name, shown trailing the label. */
  code?: string;
  hint?: ReactNode;
  error?: ReactNode;
  required?: boolean;
  /** Marks a value that differs from the definition's default. */
  changed?: boolean;
  /** Rendered at the end of the label row, e.g. a "reset" link. */
  labelAction?: ReactNode;
  className?: string;
  children: ReactNode;
}

/**
 * One parameter of a build. The label row carries everything the operator needs
 * to judge the value — whether it is required, which env var it lands in, and
 * whether they have changed it from the committed default.
 */
export function Field({
  label,
  htmlFor,
  code,
  hint,
  error,
  required,
  changed,
  labelAction,
  className,
  children,
}: FieldProps) {
  return (
    <div className={cn('grid gap-[7px]', className)}>
      {(label || code || labelAction) && (
        <div className='flex items-baseline gap-2'>
          {label && (
            <label htmlFor={htmlFor} className='font-medium text-fg'>
              {label}
              {changed && (
                <span
                  aria-label='changed from the default'
                  className='ml-[7px] inline-block h-[5px] w-[5px] bg-warning align-middle'
                />
              )}
            </label>
          )}
          {required && <span className='text-[11.5px] text-fg-subtle'>required</span>}
          {code && <code className='ml-auto font-mono text-[11px] text-fg-subtle'>{code}</code>}
          {labelAction && <span className={cn(!code && 'ml-auto')}>{labelAction}</span>}
        </div>
      )}
      {children}
      {(hint || error) && (
        <p className={cn('text-xs', error ? 'text-danger' : 'text-fg-subtle')}>{error || hint}</p>
      )}
    </div>
  );
}

export interface SwitchProps extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: ReactNode;
}

/** A labelled on/off switch — the settings and boolean-parameter control. */
export const Switch = forwardRef<HTMLInputElement, SwitchProps>(function Switch(
  { label, className, disabled, ...props },
  ref,
) {
  return (
    <label
      className={cn(
        'inline-flex cursor-pointer items-center gap-2.5',
        disabled && 'cursor-not-allowed opacity-50',
        className,
      )}
    >
      <input
        ref={ref}
        type='checkbox'
        disabled={disabled}
        className={cn(
          'relative m-0 h-[17px] w-[30px] shrink-0 cursor-pointer appearance-none bg-border transition-colors',
          'after:absolute after:left-0.5 after:top-0.5 after:h-[13px] after:w-[13px] after:bg-fg-muted after:transition-transform after:content-[""]',
          'checked:bg-fg checked:after:translate-x-[13px] checked:after:bg-bg',
        )}
        {...props}
      />
      {label && <span className='text-fg-muted'>{label}</span>}
    </label>
  );
});
