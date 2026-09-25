import { type ReactNode } from 'react';
import { NavLink } from 'react-router-dom';
import { cn } from '@/lib/cn';

interface TabItem<T extends string = string> {
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
}

const TAB_BASE =
  'inline-flex items-center gap-1.5 border-b-[1.5px] pb-2.5 pt-3 text-[13px] font-medium transition-colors -mb-px';

const tabTone = (active: boolean) =>
  active ? 'border-fg text-fg' : 'border-transparent text-fg-subtle hover:text-fg-muted';

function Count({ value }: { value: number }) {
  return <span className='font-mono text-[11px] text-fg-subtle tabular-nums'>{value}</span>;
}

/** Underlined tabs for local state — a view switch that is not a route. */
export function Tabs<T extends string = string>({
  value,
  onChange,
  items,
  className,
}: TabsProps<T>) {
  return (
    <div role='tablist' className={cn('flex gap-6 border-b border-border', className)}>
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
              TAB_BASE,
              tabTone(active),
              item.disabled && 'cursor-not-allowed opacity-50',
            )}
          >
            {item.label}
            {typeof item.count === 'number' && <Count value={item.count} />}
          </button>
        );
      })}
    </div>
  );
}

interface LinkTabItem {
  to: string;
  label: ReactNode;
  count?: number;
  /** Matches nested routes too, e.g. the Builds tab on /jobs/x/builds/412. */
  active: boolean;
}

export interface LinkTabsProps {
  items: LinkTabItem[];
  className?: string;
}

/**
 * The same tabs as links. A job's Builds / Run / Definition views are separate
 * URLs so they can be shared and reloaded, which local tab state cannot do.
 */
export function LinkTabs({ items, className }: LinkTabsProps) {
  return (
    <nav className={cn('flex gap-6', className)}>
      {items.map((item) => (
        <NavLink
          key={item.to}
          to={item.to}
          end
          aria-current={item.active ? 'page' : undefined}
          className={cn(TAB_BASE, tabTone(item.active))}
        >
          {item.label}
          {typeof item.count === 'number' && <Count value={item.count} />}
        </NavLink>
      ))}
    </nav>
  );
}
