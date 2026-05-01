import { Fragment, useMemo, useState, type RefObject } from 'react';
import { Listbox, Transition } from '@headlessui/react';
import { Check, ChevronDown } from 'lucide-react';
import {
  ALL_LOG_SEVERITIES,
  parseSlogLogLine,
  SEVERITY_LIGHT_UI_SWATCH,
  SEVERITY_STYLE,
  SOURCE_BADGE_LABEL,
  SOURCE_BADGE_TITLE,
  SOURCE_STYLE,
  type LogSeverity,
} from '@/utils/slogLogLine';
import { Spinner } from '@/components/ui';
import { cn } from '@/lib/cn';

export interface LogViewerProps {
  lines: string[];
  logBoxRef: RefObject<HTMLDivElement | null>;
  onScroll: () => void;
  hideScrollArea: boolean;
  logsLoading: boolean;
  loadingOlder: boolean;
  logHasMore: boolean;
}

export function LogViewer({
  lines: linesIn,
  logBoxRef,
  onScroll,
  hideScrollArea,
  logsLoading,
  loadingOlder,
  logHasMore,
}: LogViewerProps) {
  const lines = useMemo(() => linesIn ?? [], [linesIn]);
  const [enabled, setEnabled] = useState<LogSeverity[]>(() => [
    ...ALL_LOG_SEVERITIES,
  ]);

  const parsed = useMemo(
    () => lines.map((raw) => ({ raw, parsed: parseSlogLogLine(raw) })),
    [lines]
  );

  const enabledSet = useMemo(() => new Set(enabled), [enabled]);
  const visible = useMemo(
    () => parsed.filter((r) => enabledSet.has(r.parsed.severity)),
    [parsed, enabledSet]
  );

  const allSelected = enabled.length === ALL_LOG_SEVERITIES.length;
  const summary = allSelected
    ? 'All levels'
    : enabled.length === 0
      ? 'Nothing selected'
      : [...enabled].sort().join(', ');

  return (
    <div className='relative'>
      <Listbox
        value={enabled}
        onChange={(next: LogSeverity[]) => setEnabled(next)}
        multiple
      >
        <div className='relative mb-3 max-w-sm'>
          <Listbox.Button className='relative flex h-9 w-full items-center justify-between gap-2 rounded-md border border-border bg-surface px-3 text-left text-sm text-fg transition-colors focus:outline-none focus:ring-2 focus:ring-primary/30'>
            <span className='block truncate text-fg-muted'>
              Severity:{' '}
              <span className='font-medium text-fg'>{summary}</span>
            </span>
            <ChevronDown className='h-4 w-4 text-fg-muted' aria-hidden />
          </Listbox.Button>
          <Transition
            as={Fragment}
            leave='transition ease-in duration-75'
            leaveFrom='opacity-100'
            leaveTo='opacity-0'
          >
            <Listbox.Options className='absolute z-30 mt-1 max-h-60 w-full overflow-auto scrollbar-thin rounded-md border border-border bg-surface py-1 shadow-popover focus:outline-none'>
              {ALL_LOG_SEVERITIES.map((sev) => (
                <Listbox.Option
                  key={sev}
                  value={sev}
                  className={({ active }) =>
                    cn(
                      'relative flex cursor-pointer select-none items-center gap-2 px-3 py-2 text-sm',
                      active && 'bg-surface-hover'
                    )
                  }
                >
                  {({ selected }) => (
                    <>
                      <span className='flex h-4 w-4 items-center justify-center text-primary'>
                        {selected && <Check className='h-4 w-4' />}
                      </span>
                      <span
                        className='h-2.5 w-2.5 shrink-0 rounded-sm'
                        style={{ backgroundColor: SEVERITY_LIGHT_UI_SWATCH[sev] }}
                        aria-hidden
                      />
                      <span className='font-mono text-sm font-semibold text-fg'>
                        {sev}
                      </span>
                    </>
                  )}
                </Listbox.Option>
              ))}
            </Listbox.Options>
          </Transition>
        </div>
      </Listbox>

      {logsLoading && lines.length === 0 && (
        <div className='flex justify-center py-6'>
          <Spinner size={24} />
        </div>
      )}

      <div
        ref={logBoxRef as RefObject<HTMLDivElement>}
        onScroll={onScroll}
        className={cn(
          'max-h-[420px] overflow-auto scrollbar-thin rounded-md bg-[rgb(17_24_39)] p-3 font-mono text-xs text-slate-100',
          hideScrollArea && 'hidden'
        )}
      >
        {loadingOlder && (
          <div className='mb-2 text-[11px] text-slate-400'>Loading older…</div>
        )}
        {lines.length === 0 && !logsLoading ? (
          <div className='text-slate-400'>No log lines in this window yet.</div>
        ) : visible.length === 0 && lines.length > 0 ? (
          <div className='text-slate-400'>
            No lines match the selected severities.
            {!allSelected && ' Adjust the filter above.'}
          </div>
        ) : (
          visible.map((row, i) => {
            const { parsed: p } = row;
            const sevSt = SEVERITY_STYLE[p.severity];
            const srcSt = SOURCE_STYLE[p.source];
            return (
              <div
                key={`${i}-${row.raw.slice(0, 32)}`}
                className='grid items-start gap-x-2 gap-y-0.5 border-b border-white/5 py-1 last:border-b-0 sm:grid-cols-[minmax(140px,168px)_minmax(64px,auto)_minmax(52px,auto)_minmax(0,1fr)]'
              >
                <span
                  className='truncate text-[10.5px] text-slate-400'
                  title={p.timestampDisplay}
                >
                  {p.timestampDisplay}
                </span>
                <span
                  title={SOURCE_BADGE_TITLE[p.source]}
                  className='justify-self-start whitespace-nowrap rounded px-1.5 py-[1px] text-[9.5px] font-bold uppercase tracking-wide'
                  style={{ color: srcSt.color, backgroundColor: srcSt.labelBg }}
                >
                  {SOURCE_BADGE_LABEL[p.source]}
                </span>
                <span
                  className='justify-self-start whitespace-nowrap rounded px-1.5 py-[1px] text-[10px] font-bold uppercase tracking-wide'
                  style={{ color: sevSt.color, backgroundColor: sevSt.labelBg }}
                >
                  {p.severity}
                </span>
                <span
                  className='whitespace-pre-wrap break-all text-[11.5px] leading-[1.5]'
                  style={{ color: sevSt.color }}
                >
                  {p.message}
                </span>
              </div>
            );
          })
        )}
      </div>
      {!logHasMore && lines.length > 0 && (
        <p className='mt-2 text-xs text-fg-muted'>Start of log file</p>
      )}
    </div>
  );
}
