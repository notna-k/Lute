/**
 * Formatting helpers shared by every panel view.
 *
 * Durations and timestamps appear in tables, rails and log gutters, and they
 * must agree everywhere — "2m 04s" in one place and "124s" in another reads as
 * two different numbers. These are the single source for that.
 */

const pad2 = (n: number) => String(n).padStart(2, '0');

/** `41s`, `2m 04s`, `1h 12m`. Nullish or NaN renders as an em dash. */
export function duration(ms: number | null | undefined): string {
  if (ms == null || Number.isNaN(ms)) return '—';
  const s = Math.max(0, Math.round(ms / 1000));
  if (s < 60) return `${s}s`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ${pad2(s % 60)}s`;
  return `${Math.floor(m / 60)}h ${pad2(m % 60)}m`;
}

/** `just now`, `42s ago`, `7m ago`, `3h ago`, `2d ago`. */
export function relativeTime(ts: number | null | undefined): string {
  if (ts == null) return '—';
  const s = Math.round((Date.now() - ts) / 1000);
  if (s < 5) return 'just now';
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  const h = Math.floor(m / 60);
  if (h < 24) return `${h}h ago`;
  return `${Math.floor(h / 24)}d ago`;
}

/** Wall clock in local time: `14:32`, or `14:32:10` with `withSeconds`. */
export function clockTime(ts: number, withSeconds = false): string {
  const d = new Date(ts);
  const base = `${pad2(d.getHours())}:${pad2(d.getMinutes())}`;
  return withSeconds ? `${base}:${pad2(d.getSeconds())}` : base;
}

/** `Sep 19, 14:32` — enough to place a build without a full ISO stamp. */
export function timestamp(ts: number): string {
  const d = new Date(ts);
  const month = [
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec',
  ][d.getMonth()];
  return `${month} ${d.getDate()}, ${clockTime(ts)}`;
}

/** Elapsed time inside a build log: `0:41.2`, `12:07.9`. */
export function elapsed(seconds: number): string {
  const whole = Math.floor(seconds);
  const tenths = Math.floor((seconds % 1) * 10);
  return `${Math.floor(whole / 60)}:${pad2(whole % 60)}.${tenths}`;
}

/** `0.93` → `93%`. */
export function percent(ratio: number): string {
  return `${Math.round(ratio * 100)}%`;
}

/** Parses an API timestamp (RFC 3339 or epoch ms) into epoch ms, or null. */
export function toEpochMs(value: string | number | undefined | null): number | null {
  if (value == null || value === '') return null;
  if (typeof value === 'number') return Number.isFinite(value) ? value : null;
  const parsed = Date.parse(value);
  return Number.isNaN(parsed) ? null : parsed;
}
