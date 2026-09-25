import { type ReactNode } from 'react';
import { Listbox, ListboxButton, ListboxOption, ListboxOptions } from '@headlessui/react';
import { Check, ChevronDown } from 'lucide-react';
import { cn } from '@/lib/cn';

interface SelectOption<T extends string = string> {
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
        <ListboxButton
          id={id}
          className={cn(
            'relative flex w-full items-center justify-between gap-2 rounded-md border border-border bg-surface text-left text-fg transition-colors focus:outline-none focus:ring-2 focus:ring-primary/30 disabled:cursor-not-allowed disabled:opacity-50',
            sizeClasses,
            buttonClassName,
          )}
        >
          <span className={cn('block truncate', !current && 'text-fg-subtle')}>
            {current ? current.label : (placeholder ?? 'Select…')}
          </span>
          <ChevronDown className='h-4 w-4 text-fg-muted' aria-hidden />
        </ListboxButton>
        <ListboxOptions
          transition
          className='absolute z-50 mt-1 max-h-60 w-full overflow-auto scrollbar-thin rounded-md border border-border bg-surface py-1 shadow-popover transition duration-75 ease-in focus:outline-none data-[closed]:opacity-0'
        >
          {options.map((opt) => (
            <ListboxOption
              key={opt.value}
              value={opt.value}
              disabled={opt.disabled}
              className='relative flex cursor-pointer select-none items-center gap-2 px-3 py-2 text-sm data-[disabled]:cursor-not-allowed data-[focus]:bg-surface-hover data-[disabled]:opacity-50'
            >
              {({ selected }) => (
                <>
                  <span className='flex h-4 w-4 items-center justify-center text-primary'>
                    {selected && <Check className='h-4 w-4' />}
                  </span>
                  <span className='flex-1 truncate'>{opt.label}</span>
                </>
              )}
            </ListboxOption>
          ))}
        </ListboxOptions>
      </div>
    </Listbox>
  );
}
