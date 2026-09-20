/**
 * Parse lines written by Go log/slog JSONHandler (job-*.log).
 * Handles level as string (common) or numeric (some encodings).
 */

export type LogSeverity = 'DEBUG' | 'INFO' | 'WARN' | 'ERROR' | 'FATAL' | 'UNKNOWN';

export const ALL_LOG_SEVERITIES: LogSeverity[] = ['DEBUG', 'INFO', 'WARN', 'ERROR', 'FATAL', 'UNKNOWN'];

const LEVEL_BY_NUM: Record<number, LogSeverity> = {
    [-4]: 'DEBUG',
    0: 'INFO',
    4: 'WARN',
    8: 'ERROR',
};

/** Normalized slog `source` attribute (worker uses system | container). */
export type LogSource = 'container' | 'system' | 'unknown';

export interface ParsedSlogLine {
    severity: LogSeverity;
    /** Worker log source: container stdout vs system (shown as "noop" in UI). */
    source: LogSource;
    /** Short time for console column */
    timestampDisplay: string;
    /** Primary message */
    message: string;
    /** Original line (for keys / non-JSON) */
    raw: string;
    /** True when JSON parsed successfully */
    structured: boolean;
}

function normalizeSource(v: unknown): LogSource {
    if (typeof v !== 'string') return 'unknown';
    const s = v.trim().toLowerCase();
    if (s === 'container') return 'container';
    if (s === 'system') return 'system';
    return 'unknown';
}

function normalizeSeverity(level: unknown): LogSeverity {
    if (typeof level === 'string') {
        const u = level.trim().toUpperCase();
        if (u === 'DEBUG') return 'DEBUG';
        if (u === 'INFO') return 'INFO';
        if (u === 'WARN' || u === 'WARNING') return 'WARN';
        if (u === 'ERROR') return 'ERROR';
        if (u === 'FATAL' || u === 'CRITICAL' || u === 'PANIC') return 'FATAL';
        return 'UNKNOWN';
    }
    if (typeof level === 'number' && Number.isFinite(level)) {
        const n = level as number;
        if (n in LEVEL_BY_NUM) return LEVEL_BY_NUM[n];
        if (n > 8) return 'FATAL';
        if (n > 4) return 'ERROR';
        if (n > 0) return 'WARN';
        if (n > -4) return 'INFO';
        return 'DEBUG';
    }
    return 'UNKNOWN';
}

function formatTimestamp(timeVal: unknown): string {
    if (typeof timeVal !== 'string' || !timeVal) return '—';
    const ms = Date.parse(timeVal);
    if (Number.isNaN(ms)) return timeVal;
    try {
        const d = new Date(ms);
        const p = (n: number, len = 2) => String(n).padStart(len, '0');
        return `${d.getFullYear()}-${p(d.getMonth() + 1)}-${p(d.getDate())} ${p(d.getHours())}:${p(d.getMinutes())}:${p(d.getSeconds())}.${p(d.getMilliseconds(), 3)}`;
    } catch {
        return new Date(ms).toISOString();
    }
}

function buildMessage(msg: unknown, rest: Record<string, unknown>): string {
    let base: string;
    if (typeof msg === 'string') base = msg;
    else if (msg != null) base = JSON.stringify(msg);
    else base = '';

    const skip = new Set(['time', 'level', 'msg', 'source']);
    const parts: string[] = [];
    for (const [k, v] of Object.entries(rest)) {
        if (skip.has(k)) continue;
        const s = typeof v === 'object' && v !== null ? JSON.stringify(v) : String(v);
        parts.push(`${k}=${s}`);
    }
    if (parts.length === 0) return base;
    const suffix = parts.join(' ');
    return base ? `${base}  ${suffix}` : suffix;
}

export function parseSlogLogLine(raw: string): ParsedSlogLine {
    const trimmed = raw.trim();
    if (!trimmed.startsWith('{')) {
        return {
            severity: 'UNKNOWN',
            source: 'unknown',
            timestampDisplay: '—',
            message: raw,
            raw,
            structured: false,
        };
    }
    try {
        const obj = JSON.parse(trimmed) as Record<string, unknown>;
        const severity = normalizeSeverity(obj.level);
        const source = normalizeSource(obj.source);
        const timestampDisplay = formatTimestamp(obj.time);
        const msg = obj.msg;
        const rest = { ...obj };
        delete rest.time;
        delete rest.level;
        delete rest.source;
        delete rest.msg;
        const message = buildMessage(msg, rest);
        return {
            severity,
            source,
            timestampDisplay,
            message: message || '(empty message)',
            raw,
            structured: true,
        };
    } catch {
        return {
            severity: 'UNKNOWN',
            source: 'unknown',
            timestampDisplay: '—',
            message: raw,
            raw,
            structured: false,
        };
    }
}

/** Text and badge background per severity (dark console) */
export const SEVERITY_STYLE: Record<
    LogSeverity,
    { color: string; labelBg: string }
> = {
    FATAL: { color: '#ff5252', labelBg: 'rgba(183, 28, 28, 0.55)' }, // dark red (stronger than ERROR)
    ERROR: { color: '#ffab91', labelBg: 'rgba(211, 47, 47, 0.35)' }, // lighter red
    WARN: { color: '#fff59d', labelBg: 'rgba(245, 127, 23, 0.25)' }, // yellow
    INFO: { color: '#fafafa', labelBg: 'rgba(250, 250, 250, 0.08)' }, // white
    DEBUG: { color: '#b2ebf2', labelBg: 'rgba(128, 203, 196, 0.22)' }, // very light green-cyan / teal
    UNKNOWN: { color: '#b0bec5', labelBg: 'rgba(144, 164, 174, 0.15)' },
};

/**
 * Solid swatches for severity rows in light UI (filters, menus).
 * Do not use SEVERITY_STYLE.color there — those tints are for dark log backgrounds only.
 */
export const SEVERITY_LIGHT_UI_SWATCH: Record<LogSeverity, string> = {
    DEBUG: '#00796b',
    INFO: '#0d47a1',
    WARN: '#e65100',
    ERROR: '#c62828',
    FATAL: '#b71c1c',
    UNKNOWN: '#455a64',
};

/** Dark-console badge for log source (container vs worker/system). */
export const SOURCE_STYLE: Record<LogSource, { color: string; labelBg: string }> = {
    container: { color: '#80deea', labelBg: 'rgba(0, 151, 167, 0.38)' },
    system: { color: '#dce775', labelBg: 'rgba(130, 119, 23, 0.42)' },
    unknown: { color: '#b0bec5', labelBg: 'rgba(144, 164, 174, 0.2)' },
};

/** Short label in the log row: system logs show as "noop" (noop/container jobs). */
export const SOURCE_BADGE_LABEL: Record<LogSource, string> = {
    container: 'container',
    system: 'noop',
    unknown: 'unknown',
};

/** Tooltip: real slog source value */
export const SOURCE_BADGE_TITLE: Record<LogSource, string> = {
    container: 'source: container (stdout/stderr from job)',
    system: 'source: system (worker / noop job)',
    unknown: 'source: unknown',
};
