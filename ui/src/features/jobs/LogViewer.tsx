/**
 * The build log pane.
 *
 * Dark in both themes on purpose: log output carries its own severity colours,
 * and those are legible against ink, not paper. The pane is a self-contained
 * column — a toolbar that never scrolls and a body that does — so it can be
 * dropped into a page that already has a fixed header.
 */
import { useMemo, useState } from 'react';
import { Search, WrapText } from 'lucide-react';
import {
  ALL_LOG_SEVERITIES,
  parseSlogLogLine,
  SEVERITY_STYLE,
  SOURCE_BADGE_LABEL,
  SOURCE_BADGE_TITLE,
  SOURCE_STYLE,
  type LogSeverity,
} from '@/utils/slogLogLine';
import { Spinner } from '@/components/ui/Spinner';
import { cn } from '@/lib/cn';
import type { useJobLogs } from '@/hooks/useJobLogs';

type JobLogs = ReturnType<typeof useJobLogs>;

export interface LogViewerProps {
  logs: JobLogs;
  /** Shown in the toolbar, e.g. `builds/412.log`. */
  title?: string;
  /** Extra controls for the toolbar's right edge (download, follow, re-run). */
  actions?: React.ReactNode;
  className?: string;
}

/** Severity filters. `UNKNOWN` is folded into INFO so the row stays scannable. */
const FILTERS: { label: string; severities: LogSeverity[] }[] = [
  { label: 'All', severities: [...ALL_LOG_SEVERITIES] },
  { label: 'Warn+', severities: ['WARN', 'ERROR', 'FATAL'] },
  { label: 'Errors', severities: ['ERROR', 'FATAL'] },
];

export function LogViewer({ logs, title, actions, className }: LogViewerProps) {
  const { boxRef, lines, loading, loadingOlder, hasMore, error, onScroll } = logs;
  const [filter, setFilter] = useState(0);
  const [needle, setNeedle] = useState('');
  const [wrap, setWrap] = useState(true);

  const rows = useMemo(() => {
    const allowed = new Set(FILTERS[filter].severities);
    const q = needle.trim().toLowerCase();
    return lines
      .map((raw, i) => ({ raw, n: i + 1, p: parseSlogLogLine(raw) }))
      .filter((row) => allowed.has(row.p.severity) && (!q || row.raw.toLowerCase().includes(q)));
  }, [lines, filter, needle]);

  return (
    <div className={cn('flex min-h-0 flex-col bg-log-bg', className)}>
      <div className='flex shrink-0 flex-wrap items-center gap-2 border-b border-log-line px-3 py-2'>
        {title && <span className='font-mono text-[11.5px] text-log-dim'>{title}</span>}
        <div className='inline-flex border border-log-line'>
          {FILTERS.map((f, i) => (
            <button
              key={f.label}
              type='button'
              aria-pressed={filter === i}
              onClick={() => setFilter(i)}
              className={cn(
                'h-[22px] px-2 text-[11.5px] transition-colors',
                i > 0 && 'border-l border-log-line',
                filter === i ? 'bg-log-line text-log-fg' : 'text-log-dim hover:text-log-fg',
              )}
            >
              {f.label}
            </button>
          ))}
        </div>
        <label className='relative'>
          <Search className='pointer-events-none absolute left-2 top-1/2 h-3 w-3 -translate-y-1/2 text-log-dim' />
          <input
            value={needle}
            onChange={(e) => setNeedle(e.target.value)}
            placeholder='Find in log'
            aria-label='Find in log'
            className='h-[22px] w-40 border border-log-line bg-transparent pl-7 pr-2 font-mono text-[11.5px] text-log-fg placeholder:text-log-dim focus:border-log-dim focus:outline-none'
          />
        </label>
        <button
          type='button'
          aria-pressed={wrap}
          onClick={() => setWrap((w) => !w)}
          title='Wrap long lines'
          className={cn(
            'inline-flex h-[22px] items-center gap-1 border border-log-line px-2 text-[11.5px] transition-colors',
            wrap ? 'bg-log-line text-log-fg' : 'text-log-dim hover:text-log-fg',
          )}
        >
          <WrapText className='h-3 w-3' /> wrap
        </button>
        <span className='ml-auto flex items-center gap-2.5 font-mono text-[11.5px] text-log-dim tabular-nums'>
          {error && <span className='text-danger'>{error}</span>}
          {rows.length}/{lines.length} lines
          {actions}
        </span>
      </div>

      <div
        ref={boxRef}
        onScroll={onScroll}
        className='scrollbar-thin min-h-0 flex-1 overflow-auto py-1 font-mono text-[12px] leading-[1.55]'
      >
        {loading && lines.length === 0 ? (
          <div className='flex justify-center py-8'>
            <Spinner size={20} />
          </div>
        ) : (
          <>
            {loadingOlder && (
              <div className='px-3 py-1 text-[11px] text-log-dim'>Loading older…</div>
            )}
            {!hasMore && lines.length > 0 && (
              <div className='px-3 py-1 text-[11px] text-log-dim'>— start of log —</div>
            )}
            {rows.length === 0 && (
              <div className='px-3 py-6 text-[12px] text-log-dim'>
                {lines.length === 0 ? 'No output yet.' : 'No lines match the filter.'}
              </div>
            )}
            {rows.map((row) => {
              const sev = SEVERITY_STYLE[row.p.severity];
              const src = SOURCE_STYLE[row.p.source];
              return (
                <div
                  key={`${row.n}-${row.raw.slice(0, 24)}`}
                  className='group flex gap-2.5 px-3 hover:bg-white/[0.04]'
                >
                  <span className='w-9 shrink-0 select-none text-right text-log-dim tabular-nums'>
                    {row.n}
                  </span>
                  <span
                    className='w-[86px] shrink-0 truncate text-log-dim'
                    title={row.p.timestampDisplay}
                  >
                    {row.p.timestampDisplay}
                  </span>
                  <span
                    title={SOURCE_BADGE_TITLE[row.p.source]}
                    className='w-[62px] shrink-0 truncate text-[10.5px] uppercase'
                    style={{ color: src.color }}
                  >
                    {SOURCE_BADGE_LABEL[row.p.source]}
                  </span>
                  <span
                    className='w-[42px] shrink-0 text-[10.5px] font-semibold uppercase'
                    style={{ color: sev.color }}
                  >
                    {row.p.severity === 'UNKNOWN' ? '' : row.p.severity}
                  </span>
                  <span
                    className={cn(
                      'min-w-0 flex-1',
                      wrap ? 'whitespace-pre-wrap break-all' : 'whitespace-pre',
                    )}
                    style={{ color: sev.color }}
                  >
                    {row.p.message || ' '}
                  </span>
                </div>
              );
            })}
          </>
        )}
      </div>
    </div>
  );
}
