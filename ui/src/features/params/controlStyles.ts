import { useEffect, useRef } from 'react';
import type { ParameterOption } from '@/types/jobs';

// Parameter values are identifiers and paths, so they read best fixed-width.
export const FIELD_BASE =
  'w-full border bg-bg px-2.5 py-[6px] font-mono text-[12.5px] text-fg placeholder:text-fg-subtle transition-colors focus-visible:outline-none disabled:cursor-not-allowed disabled:opacity-60';

export function ring(invalid?: boolean) {
  return invalid
    ? 'border-danger'
    : 'border-border hover:border-border-strong focus-visible:border-fg';
}

/** Border for a control that opens a popover. */
export function popoverRing(open: boolean, invalid?: boolean, hover = true) {
  if (invalid) return 'border-danger';
  if (open) return 'border-primary ring-2 ring-primary/20';
  return hover ? 'border-border hover:border-border-strong' : 'border-border';
}

export function labelOf(o: ParameterOption) {
  return o.label || o.value;
}

/** Closes a popover on an outside click or Escape. */
export function useDismiss(onClose: () => void) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => {
    function onDown(e: MouseEvent) {
      if (ref.current && !ref.current.contains(e.target as Node)) onClose();
    }
    function onKey(e: KeyboardEvent) {
      if (e.key === 'Escape') onClose();
    }
    document.addEventListener('mousedown', onDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [onClose]);
  return ref;
}
