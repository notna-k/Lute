import { Fragment, type ReactNode } from 'react';
import { Listbox, Transition } from '@headlessui/react';
import { Check, ChevronDown } from 'lucide-react';
import { cn } from '@/lib/cn';

export interface SelectOption<T extends string = string> {
  value: T;
  label: ReactNode;
  disabled?: boolean;
}

export interface SelectProps<T extends string = string> {
  value: T;
  onChange: (value: T) => void;
  options: SelectOption<T>[];
  placeholder?: string;
  className?: string;
  buttonClassName?: string;
  disabled?: boolean;
  size?: 'sm' | 'md';
  id?: string;
}

export function Select<T extends string = string>({
  value,
  onChange,
  options,
  placeholder,
  className,
  buttonClassName,
  disabled,
  size = 'md',
  id,
}: SelectProps<T>) {
  const current = options.find((o) => o.value === value);
  const sizeClasses = size === 'sm' ? 'h-8 text-sm px-2.5' : 'h-9 text-sm px-3';

  return (
    <Listbox value={value} onChange={onChange} disabled={disabled}>
      <div className={cn('relative', className)}>
        <Listbox.Button
          id={id}
          className={cn(
            'relative flex w-full items-center justify-between gap-2 rounded-md border border-border bg-surface text-left text-fg transition-colors focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:cursor-not-allowed disabled:opacity-50',
            sizeClasses,
            buttonClassName
          )}
        >
          <span className={cn('block truncate', !current && 'text-fg-subtle')}>
            {current ? current.label : placeholder ?? 'Select…'}
          </span>
          <ChevronDown
            className='h-4 w-4 text-fg-muted'
            aria-hidden
          />
        </Listbox.Button>
        <Transition
          as={Fragment}
          leave='transition ease-in duration-75'
          leaveFrom='opacity-100'
          leaveTo='opacity-0'
        >
          <Listbox.Options className='absolute z-50 mt-1 max-h-60 w-full overflow-auto scrollbar-thin rounded-md border border-border bg-surface py-1 shadow-popover focus:outline-none'>
            {options.map((opt) => (
              <Listbox.Option
                key={opt.value}
                value={opt.value}
                disabled={opt.disabled}
                className={({ active }) =>
                  cn(
                    'relative flex cursor-pointer select-none items-center gap-2 px-3 py-2 text-sm',
                    active && 'bg-surface-hover',
                    opt.disabled && 'cursor-not-allowed opacity-50'
                  )
                }
              >
                {({ selected }) => (
                  <>
                    <span
                      className={cn(
                        'flex h-4 w-4 items-center justify-center text-primary'
                      )}
                    >
                      {selected && <Check className='h-4 w-4' />}
                    </span>
                    <span className='flex-1 truncate'>{opt.label}</span>
                  </>
                )}
              </Listbox.Option>
            ))}
          </Listbox.Options>
        </Transition>
      </div>
    </Listbox>
  );
}
