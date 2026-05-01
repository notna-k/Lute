import type { BadgeTone } from '@/components/ui';
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
