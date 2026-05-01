import { type ReactNode } from 'react';
import { cn } from '@/lib/cn';

export interface TabItem<T extends string = string> {
  value: T;
  label: ReactNode;
  count?: number;
  disabled?: boolean;
}

export interface TabsProps<T extends string = string> {
  value: T;
  onChange: (value: T) => void;
  items: TabItem<T>[];
  className?: string;
  variant?: 'underline' | 'pill';
}

export function Tabs<T extends string = string>({
  value,
  onChange,
  items,
  className,
  variant = 'underline',
}: TabsProps<T>) {
  if (variant === 'pill') {
    return (
      <div
        role='tablist'
        className={cn(
          'inline-flex items-center gap-1 rounded-md border border-border bg-bg-subtle p-1',
          className
        )}
      >
        {items.map((item) => {
          const active = item.value === value;
          return (
            <button
              key={item.value}
              type='button'
              role='tab'
              aria-selected={active}
              disabled={item.disabled}
              onClick={() => onChange(item.value)}
              className={cn(
                'inline-flex items-center gap-1.5 rounded px-3 py-1.5 text-sm font-medium transition-colors',
                active
                  ? 'bg-surface text-fg shadow-sm'
                  : 'text-fg-muted hover:text-fg',
                item.disabled && 'cursor-not-allowed opacity-50'
              )}
            >
              {item.label}
              {typeof item.count === 'number' && (
                <span
                  className={cn(
                    'rounded-full px-1.5 py-px text-xxs font-semibold',
                    active
                      ? 'bg-primary text-fg-onPrimary'
                      : 'bg-bg-muted text-fg-muted'
                  )}
                >
                  {item.count}
                </span>
              )}
            </button>
          );
        })}
      </div>
    );
  }

  return (
    <div
      role='tablist'
      className={cn('flex border-b border-border', className)}
    >
      {items.map((item) => {
        const active = item.value === value;
        return (
          <button
            key={item.value}
            type='button'
            role='tab'
            aria-selected={active}
            disabled={item.disabled}
            onClick={() => onChange(item.value)}
            className={cn(
              'relative inline-flex items-center gap-1.5 px-4 py-2.5 text-sm font-medium transition-colors',
              active
                ? 'text-fg'
                : 'text-fg-muted hover:text-fg',
              item.disabled && 'cursor-not-allowed opacity-50'
            )}
          >
            {item.label}
            {typeof item.count === 'number' && (
              <span
                className={cn(
                  'rounded-full px-1.5 py-px text-xxs font-semibold',
                  active
                    ? 'bg-primary-subtle text-info-fg'
                    : 'bg-bg-muted text-fg-muted'
                )}
              >
                {item.count}
              </span>
            )}
            {active && (
              <span
                aria-hidden
                className='absolute inset-x-3 bottom-0 h-0.5 rounded-full bg-primary'
              />
            )}
          </button>
        );
      })}
    </div>
  );
}
