import type { WorkerState } from '@/components/ui/Status';
import type { Worker } from '@/types';

/** Maps the registry's worker status onto the shared status vocabulary. */
export function workerState(status: Worker['status']): WorkerState {
  switch (status) {
    case 'alive':
    case 'registered':
      return 'idle';
    case 'pending':
    case 'deleting':
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

/** The `docker run` that starts a worker on a rootless Docker host. */
/** Hides a secret token on screen, keeping the start the panel lists it by. */
export function maskToken(token: string, visible = 12) {
  return token.length <= visible ? token : token.slice(0, visible) + '*'.repeat(16);
}

export function runCommand(opts: { server: string; image: string; token: string; name?: string }) {
  const lines = [
    'docker run -d --name lute-worker --restart unless-stopped \\',
    '  --stop-timeout 1800 \\',
    '  -v "$XDG_RUNTIME_DIR/docker.sock:/var/run/docker.sock" \\',
    '  -v "$HOME/.local/share/lute-worker:/var/lib/lute-worker" \\',
    `  -e LUTE_SERVER=${opts.server} \\`,
    `  -e LUTE_TOKEN=${opts.token} \\`,
  ];
  if (opts.name) lines.push(`  -e LUTE_NAME=${opts.name} \\`);
  lines.push(`  ${opts.image}`);
  return lines.join('\n');
}

/** The engine's resource limits that jobs cannot get on this worker. */
export function missingLimits(w: Worker): string[] {
  const e = w.engine;
  if (!e) return [];
  return [!e.memory_limit && 'memory', !e.cpu_limit && 'cpu', !e.pids_limit && 'pids'].filter(
    Boolean,
  ) as string[];
}
