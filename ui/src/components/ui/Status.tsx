import { type ReactNode } from 'react';
import { cn } from '@/lib/cn';

/**
 * The status vocabulary.
 *
 * Every build state has one shape and one colour, and they are declared here
 * once so a marker means the same thing in a table, a rail, a log header and a
 * toast. Shape carries the meaning as much as colour does — a diamond reads as
 * "failed" for a viewer who cannot separate red from green.
 */

export type BuildState = 'passed' | 'failed' | 'running' | 'queued' | 'aborted';
export type WorkerState = 'idle' | 'busy' | 'draining' | 'offline';
type State = BuildState | WorkerState;

const BUILD_STATE_LABEL: Record<BuildState, string> = {
  passed: 'Passed',
  failed: 'Failed',
  running: 'Running',
  queued: 'Queued',
  aborted: 'Cancelled',
};

const WORKER_STATE_LABEL: Record<WorkerState, string> = {
  idle: 'Idle',
  busy: 'Busy',
  draining: 'Draining',
  offline: 'Offline',
};

function stateLabel(state: State): string {
  return (
    (BUILD_STATE_LABEL as Record<string, string>)[state] ??
    (WORKER_STATE_LABEL as Record<string, string>)[state] ??
    state
  );
}

const MARK: Record<State, string> = {
  passed: 'bg-success',
  failed: 'bg-danger rotate-45 scale-[0.85]',
  running: 'border-[1.5px] border-warning',
  queued: 'border-[1.5px] border-dashed border-fg-subtle',
  aborted: 'bg-fg-subtle',
  idle: 'bg-success',
  busy: 'border-[1.5px] border-warning',
  draining: 'border-[1.5px] border-warning',
  offline: 'border-[1.5px] border-fg-subtle',
};

export interface StatusMarkProps {
  state: State;
  /** Edge length in px. 10 suits body rows; 8 suits dense log headers. */
  size?: number;
  className?: string;
}

/** The bare glyph, for use inside links, table cells and headings. */
export function StatusMark({ state, size = 10, className }: StatusMarkProps) {
  const filled = state === 'running' || state === 'busy';
  return (
    <span
      aria-hidden
      className={cn('relative inline-block shrink-0', MARK[state], className)}
      style={{ width: size, height: size }}
    >
      {filled && (
        <span
          className={cn(
            'absolute inset-[1.5px] bg-warning',
            state === 'running' && 'animate-breathe motion-reduce:animate-none',
          )}
        />
      )}
      {state === 'aborted' && (
        <span className='absolute inset-x-px top-1/2 h-[2px] -translate-y-1/2 bg-bg' />
      )}
    </span>
  );
}

const TEXT_TONE: Record<State, string> = {
  passed: 'text-success',
  failed: 'text-danger',
  running: 'text-warning',
  queued: 'text-fg-subtle',
  aborted: 'text-fg-subtle',
  idle: 'text-success',
  busy: 'text-warning',
  draining: 'text-warning',
  offline: 'text-fg-subtle',
};

export interface StatusTextProps {
  state: State;
  /** Defaults to the canonical label; pass a node for "#412" or "queued 3m". */
  children?: ReactNode;
  className?: string;
}

/** Marker plus tinted label — the inline form used in table cells. */
export function StatusText({ state, children, className }: StatusTextProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-[7px] whitespace-nowrap font-medium',
        TEXT_TONE[state],
        className,
      )}
    >
      <StatusMark state={state} />
      {children ?? stateLabel(state)}
    </span>
  );
}

const BADGE_TONE: Record<State, string> = {
  passed: 'bg-success-subtle text-success',
  failed: 'bg-danger-subtle text-danger',
  running: 'bg-warning-subtle text-warning',
  queued: 'border border-border bg-bg-subtle text-fg-muted',
  aborted: 'border border-border bg-bg-subtle text-fg-muted',
  idle: 'bg-success-subtle text-success',
  busy: 'bg-warning-subtle text-warning',
  draining: 'bg-warning-subtle text-warning',
  offline: 'border border-border bg-bg-subtle text-fg-muted',
};

/** The filled form, for the one place a page states its subject's status. */
export function StatusBadge({ state, children, className }: StatusTextProps) {
  return (
    <span
      className={cn(
        'inline-flex h-[22px] items-center gap-[7px] whitespace-nowrap px-2 text-xs font-medium',
        BADGE_TONE[state],
        className,
      )}
    >
      <StatusMark state={state} />
      {children ?? stateLabel(state)}
    </span>
  );
}
