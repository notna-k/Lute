import { forwardRef, type InputHTMLAttributes, type ReactNode } from 'react';
import { Check } from 'lucide-react';
import { cn } from '@/lib/cn';

export interface CheckboxProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, 'type'> {
  label?: ReactNode;
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  function Checkbox({ className, label, id, ...props }, ref) {
    return (
      <label
        htmlFor={id}
        className={cn(
          'inline-flex cursor-pointer select-none items-center gap-2',
          props.disabled && 'cursor-not-allowed opacity-50',
          className
        )}
      >
        <span className='relative inline-flex h-3.5 w-3.5 items-center justify-center'>
          <input
            ref={ref}
            id={id}
            type='checkbox'
            className='peer h-3.5 w-3.5 cursor-pointer appearance-none border border-border bg-surface transition-colors checked:border-fg checked:bg-fg'
            {...props}
          />
          <Check
            aria-hidden
            className='pointer-events-none absolute h-2.5 w-2.5 text-bg opacity-0 peer-checked:opacity-100'
          />
        </span>
        {label && <span className='text-[13px] text-fg'>{label}</span>}
      </label>
    );
  }
);
