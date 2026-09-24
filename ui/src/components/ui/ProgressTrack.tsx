import { cn } from '@/lib/cn';
import type { BuildState } from './Status';

export interface ProgressTrackProps {
  /** 0..1 of the expected duration. Clamped, since builds can overrun. */
  value: number;
  state?: BuildState;
  /** Marks the expected-finish point, so an overrun is visible as it happens. */
  showTarget?: boolean;
  className?: string;
}

const FILL: Partial<Record<BuildState, string>> = {
  running: 'bg-warning',
  failed: 'bg-danger',
  passed: 'bg-fg-subtle',
  aborted: 'bg-fg-subtle',
};

/** A thin elapsed-vs-usual bar, used in build headers and worker slot rows. */
export function ProgressTrack({
  value,
  state = 'running',
  showTarget,
  className,
}: ProgressTrackProps) {
  const ratio = Math.max(0, Math.min(1, value));
  return (
    <span className={cn('relative block h-1 bg-border', className)}>
      <span
        className={cn(
          'absolute inset-y-0 left-0 transition-[width] duration-700 ease-linear',
          FILL[state] ?? 'bg-fg-subtle',
        )}
        style={{ width: `${ratio * 100}%` }}
      />
      {showTarget && (
        <span aria-hidden className='absolute -top-0.5 -bottom-0.5 right-0 w-px bg-fg-subtle' />
      )}
    </span>
  );
}
