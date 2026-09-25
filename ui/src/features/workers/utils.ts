import type { BadgeTone } from '@/components/ui/Badge';
import type { WorkerState } from '@/components/ui/Status';
import type { Worker } from '@/types';

export function statusTone(status: Worker['status']): BadgeTone {
  switch (status) {
    case 'alive':
    case 'running':
      return 'success';
    case 'dead':
    case 'stopped':
      return 'danger';
    case 'pending':
    case 'paused':
      return 'warning';
    default:
      return 'neutral';
  }
}

export function workerInitials(name: string): string {
  return (
    name
      .split(/\s+/)
      .map((s) => s[0])
      .filter(Boolean)
      .slice(0, 2)
      .join('')
      .toUpperCase() || '?'
  );
}

/**
 * Maps the registry's worker status onto the panel's status vocabulary, so a
 * worker row and a build row use the same shapes for the same meaning.
 */
export function workerState(status: Worker['status']): WorkerState {
  switch (status) {
    case 'running':
    case 'alive':
      return 'idle';
    case 'pending':
    case 'paused':
      return 'draining';
    default:
      return 'offline';
  }
}

/** A worker's numeric metric, or null when it has not reported one. */
export function metric(w: Worker, key: string): number | null {
  const raw = w.metrics?.[key];
  const value = typeof raw === 'string' ? Number(raw) : raw;
  return typeof value === 'number' && Number.isFinite(value) ? value : null;
}
