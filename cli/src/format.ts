// Human-readable bits shared by commands: short ids, times, statuses and tables.

import type { Run, RunStatus } from './client.ts';
import { color } from './output.ts';

/** shortId is the 8-character form the panel shows and every command accepts. */
export function shortId(id: string): string {
  return id.slice(0, 8);
}

/** A ref may be written "#a1b2c3d4", as the CLI prints it. */
export function cleanRef(ref: string): string {
  return ref.trim().replace(/^#/, '');
}

export function isFinal(status: RunStatus): boolean {
  return status === 'done' || status === 'failed' || status === 'dead' || status === 'unknown';
}

export function passed(run: Run): boolean {
  return run.status === 'done';
}

export function statusText(status: RunStatus): string {
  switch (status) {
    case 'done':
      return color('green', status);
    case 'failed':
    case 'dead':
      return color('red', status);
    case 'running':
      return color('cyan', status);
    default:
      return color('dim', status);
  }
}

export function duration(ms: number | undefined): string {
  if (ms === undefined || ms <= 0) return '—';
  const s = Math.round(ms / 1000);
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${s % 60}s`;
  return `${Math.floor(m / 60)}h ${m % 60}m`;
}

export function ago(iso: string | undefined, now = Date.now()): string {
  if (!iso) return '—';
  const s = Math.round((now - Date.parse(iso)) / 1000);
  if (Number.isNaN(s)) return '—';
  if (s < 60) return `${Math.max(s, 0)}s ago`;
  if (s < 3600) return `${Math.floor(s / 60)}m ago`;
  if (s < 86400) return `${Math.floor(s / 3600)}h ago`;
  return `${Math.floor(s / 86400)}d ago`;
}

/** runLink is the run's page in the panel; raw runs have none. */
export function runLink(baseUrl: string, run: Run): string | undefined {
  return run.job ? `${baseUrl}/jobs/${encodeURIComponent(run.job)}/builds/${shortId(run.id)}` : undefined;
}

/** summary is the one line that ends a waited-for run. */
export function summary(run: Run): string {
  const id = `#${shortId(run.id)}`;
  if (run.status === 'done') return `${color('green', '✓')} Run ${id} passed in ${duration(run.elapsed_ms)}`;
  if (run.status === 'dead' && run.error === 'cancelled') return `${color('red', '✗')} Run ${id} was cancelled`;
  const why = run.error ? `: ${run.error}` : '';
  return `${color('red', '✗')} Run ${id} ${run.status === 'unknown' ? 'ended in an unknown state' : 'failed'}${why}`;
}

/** table pads columns to their widest cell; styled cells are measured without escapes. */
export function table(header: string[], rows: string[][]): string[] {
  const visible = (s: string) => s.replace(/\x1b\[[0-9;]*m/g, '').length;
  const widths = header.map((h, i) => Math.max(h.length, ...rows.map((r) => visible(r[i] ?? ''))));
  const line = (cells: string[]) =>
    cells
      .map((c, i) => (i === cells.length - 1 ? c : c + ' '.repeat((widths[i] ?? 0) - visible(c))))
      .join('  ')
      .trimEnd();
  return [color('dim', line(header)), ...rows.map(line)];
}
