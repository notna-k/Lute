import { type ReactNode } from 'react';
import { cn } from '@/lib/cn';

export interface KbdProps {
  children: ReactNode;
  className?: string;
}

/** A single key cap. Compose several for a chord: `<Kbd>Ctrl</Kbd><Kbd>K</Kbd>`. */
export function Kbd({ children, className }: KbdProps) {
  return (
    <kbd
      className={cn(
        'inline-block min-w-4 border border-border px-1 text-center font-mono text-[10.5px] font-medium leading-4 text-fg-subtle',
        className
      )}
    >
      {children}
    </kbd>
  );
}

/** True on Apple platforms, so shortcut hints show ⌘ rather than Ctrl. */
export function isAppleOS(): boolean {
  if (typeof navigator === 'undefined') return false;
  return /mac|iphone|ipad|ipod/i.test(navigator.platform || navigator.userAgent);
}

/** The platform's primary modifier, for display only. */
export const MOD_KEY = isAppleOS() ? '⌘' : 'Ctrl';
