import {
  forwardRef,
  type InputHTMLAttributes,
  type TextareaHTMLAttributes,
  type SelectHTMLAttributes,
  type ReactNode,
} from 'react';
import { cn } from '@/lib/cn';

const BASE_FIELD =
  'flex w-full rounded-md border border-border bg-surface px-3 text-sm text-fg transition-colors placeholder:text-fg-subtle focus:border-primary focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:cursor-not-allowed disabled:opacity-50';

export interface InputProps extends InputHTMLAttributes<HTMLInputElement> {
  leftIcon?: ReactNode;
  rightIcon?: ReactNode;
}

export const Input = forwardRef<HTMLInputElement, InputProps>(function Input(
  { className, leftIcon, rightIcon, ...props },
  ref
) {
  if (leftIcon || rightIcon) {
    return (
      <div className='relative w-full'>
        {leftIcon && (
          <span className='pointer-events-none absolute left-3 top-1/2 -translate-y-1/2 text-fg-muted'>
            {leftIcon}
          </span>
        )}
        <input
          ref={ref}
          className={cn(
            BASE_FIELD,
            'h-9',
            leftIcon && 'pl-9',
            rightIcon && 'pr-9',
            className
          )}
          {...props}
        />
        {rightIcon && (
          <span className='absolute right-3 top-1/2 -translate-y-1/2 text-fg-muted'>
            {rightIcon}
          </span>
        )}
      </div>
    );
  }
  return (
    <input
      ref={ref}
      className={cn(BASE_FIELD, 'h-9', className)}
      {...props}
    />
  );
});

export const Textarea = forwardRef<
  HTMLTextAreaElement,
  TextareaHTMLAttributes<HTMLTextAreaElement>
>(function Textarea({ className, ...props }, ref) {
  return (
    <textarea
      ref={ref}
      className={cn(BASE_FIELD, 'min-h-[80px] py-2', className)}
      {...props}
    />
  );
});

export interface NativeSelectProps
  extends SelectHTMLAttributes<HTMLSelectElement> {
  placeholder?: string;
}

export const NativeSelect = forwardRef<HTMLSelectElement, NativeSelectProps>(
  function NativeSelect({ className, children, ...props }, ref) {
    return (
      <select
        ref={ref}
        className={cn(BASE_FIELD, 'h-9 pr-8 appearance-none bg-[length:16px] bg-no-repeat bg-[right_10px_center]', className)}
        style={{
          backgroundImage:
            "url(\"data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' width='16' height='16' fill='none' stroke='%2394a3b8' stroke-width='2' stroke-linecap='round' stroke-linejoin='round' viewBox='0 0 24 24'%3E%3Cpath d='M6 9l6 6 6-6'/%3E%3C/svg%3E\")",
        }}
        {...props}
      >
        {children}
      </select>
    );
  }
);

export interface FieldProps {
  label?: string;
  htmlFor?: string;
  hint?: string;
  error?: string;
  required?: boolean;
  className?: string;
  children: ReactNode;
}

export function Field({
  label,
  htmlFor,
  hint,
  error,
  required,
  className,
  children,
}: FieldProps) {
  return (
    <div className={cn('flex flex-col gap-1.5', className)}>
      {label && (
        <label
          htmlFor={htmlFor}
          className='text-sm font-medium text-fg flex items-center gap-1'
        >
          {label}
          {required && <span className='text-danger'>*</span>}
        </label>
      )}
      {children}
      {(hint || error) && (
        <p
          className={cn(
            'text-xs',
            error ? 'text-danger' : 'text-fg-muted'
          )}
        >
          {error || hint}
        </p>
      )}
    </div>
  );
}
