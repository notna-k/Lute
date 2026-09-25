import { cn } from '@/lib/cn';
import type { BuildState } from './Status';

const BAR: Record<BuildState, string> = {
  passed: 'bg-fg-subtle/45',
  failed: 'bg-danger',
  running: 'bg-warning',
  queued: 'outline outline-1 -outline-offset-1 outline-dashed outline-fg-subtle',
  aborted: 'bg-fg-subtle/90',
};

export interface TapeProps {
  /** Oldest first, as read left to right. */
  states: BuildState[];
  /** Trailing window to show; older entries are dropped. */
  limit?: number;
  className?: string;
}

/** Recent history as a strip of bars, oldest left. Passes stay quiet so failures stand out. */
export function Tape({ states, limit = 16, className }: TapeProps) {
  const shown = states.slice(-limit);
  if (!shown.length) return null;
  return (
    <span
      className={cn('inline-flex h-3 items-stretch gap-px align-middle', className)}
      title={`Last ${shown.length} builds`}
    >
      {shown.map((state, i) => (
        <span key={i} aria-hidden className={cn('w-1', BAR[state])} />
      ))}
    </span>
  );
}
