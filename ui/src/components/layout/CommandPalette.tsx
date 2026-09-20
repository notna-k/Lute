import { Fragment, useEffect, useMemo, useRef, useState } from 'react';
import { Dialog, Transition } from '@headlessui/react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { ChevronRight, Play, Search, Server } from 'lucide-react';
import { listJobs } from '@/services/jobDefService';
import { useUserWorkers } from '@/hooks/useWorkers';
import { Kbd } from '@/components/ui';
import { cn } from '@/lib/cn';
import { NAV_ITEMS } from './nav';

interface Command {
  id: string;
  /** Results are shown under this heading, in first-seen order. */
  group: string;
  label: string;
  hint?: string;
  to: string;
  icon: 'page' | 'run' | 'worker';
  mono?: boolean;
}

export interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
}

const ICONS = {
  page: <ChevronRight className='h-3.5 w-3.5 text-fg-subtle' />,
  run: <Play className='h-3.5 w-3.5 text-fg-subtle' />,
  worker: <Server className='h-3.5 w-3.5 text-fg-subtle' />,
};

/**
 * The keyboard route to anything: a page, a job, a job's run form, a worker.
 *
 * It is the only navigation that reaches individual records, which is why the
 * rail can stay as short as it is.
 */
export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const [cursor, setCursor] = useState(0);
  const listRef = useRef<HTMLDivElement>(null);

  // Only fetched while the palette is open.
  const { data: jobs } = useQuery({
    queryKey: ['jobs'],
    queryFn: listJobs,
    enabled: open,
  });
  const { data: workers } = useUserWorkers({ enabled: open });

  const commands = useMemo<Command[]>(() => {
    const items: Command[] = NAV_ITEMS.map((item) => ({
      id: `page:${item.to}`,
      group: 'Go to',
      label: item.label,
      to: item.to,
      icon: 'page',
    }));
    for (const job of jobs ?? []) {
      items.push({
        id: `job:${job.slug}`,
        group: 'Jobs',
        label: job.name,
        hint: job.queue,
        to: `/jobs/${job.slug}`,
        icon: 'page',
        mono: true,
      });
    }
    for (const job of jobs ?? []) {
      items.push({
        id: `run:${job.slug}`,
        group: 'Run',
        label: `Run ${job.name}`,
        to: `/jobs/${job.slug}/run`,
        icon: 'run',
        mono: true,
      });
    }
    for (const worker of workers ?? []) {
      items.push({
        id: `worker:${worker.id}`,
        group: 'Workers',
        label: worker.name,
        hint: worker.status,
        to: `/workers/${worker.id}`,
        icon: 'worker',
        mono: true,
      });
    }
    return items;
  }, [jobs, workers]);

  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    // With no query, "Run x" for every job would bury the rest of the list.
    if (!q) return commands.filter((c) => c.group !== 'Run').slice(0, 30);
    return commands
      .filter((c) => `${c.label} ${c.hint ?? ''}`.toLowerCase().includes(q))
      .slice(0, 40);
  }, [commands, query]);

  useEffect(() => setCursor(0), [query, open]);
  useEffect(() => {
    if (!open) setQuery('');
  }, [open]);

  // Keep the highlighted row in view while arrowing through a long list.
  useEffect(() => {
    listRef.current
      ?.querySelector('[aria-selected="true"]')
      ?.scrollIntoView({ block: 'nearest' });
  }, [cursor]);

  function run(command: Command | undefined) {
    if (!command) return;
    onClose();
    navigate(command.to);
  }

  function onKeyDown(event: React.KeyboardEvent) {
    if (event.key === 'ArrowDown') {
      event.preventDefault();
      setCursor((c) => Math.min(c + 1, results.length - 1));
    } else if (event.key === 'ArrowUp') {
      event.preventDefault();
      setCursor((c) => Math.max(c - 1, 0));
    } else if (event.key === 'Enter') {
      event.preventDefault();
      run(results[cursor]);
    }
  }

  let lastGroup = '';

  return (
    <Transition.Root show={open} as={Fragment}>
      <Dialog as='div' className='relative z-50' onClose={onClose}>
        <Transition.Child
          as={Fragment}
          enter='ease-out duration-150'
          enterFrom='opacity-0'
          enterTo='opacity-100'
          leave='ease-in duration-100'
          leaveFrom='opacity-100'
          leaveTo='opacity-0'
        >
          <div className='fixed inset-0 bg-black/45' aria-hidden />
        </Transition.Child>

        <div className='fixed inset-0 overflow-y-auto px-4 pb-4 pt-[12vh]'>
          <Transition.Child
            as={Fragment}
            enter='ease-out duration-150'
            enterFrom='opacity-0 translate-y-1'
            enterTo='opacity-100 translate-y-0'
            leave='ease-in duration-100'
            leaveFrom='opacity-100'
            leaveTo='opacity-0'
          >
            <Dialog.Panel className='mx-auto w-full max-w-[620px] border border-border bg-surface shadow-overlay'>
              <Dialog.Title className='sr-only'>Command palette</Dialog.Title>
              <div className='flex h-[46px] items-center gap-2.5 border-b border-border px-3.5 text-fg-subtle'>
                <Search className='h-4 w-4 shrink-0' />
                <input
                  autoFocus
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={onKeyDown}
                  placeholder='Jobs, workers, actions…'
                  autoComplete='off'
                  className='w-full bg-transparent text-sm text-fg outline-none'
                />
                <Kbd>Esc</Kbd>
              </div>

              <div ref={listRef} className='scrollbar-thin max-h-[380px] overflow-y-auto py-1'>
                {results.length === 0 ? (
                  <p className='px-3.5 py-7 text-center text-[13px] text-fg-subtle'>
                    Nothing matches “{query}”.
                  </p>
                ) : (
                  results.map((command, i) => {
                    const heading = command.group !== lastGroup ? command.group : null;
                    lastGroup = command.group;
                    return (
                      <Fragment key={command.id}>
                        {heading && (
                          <div className='caption px-3.5 pb-1 pt-2'>{heading}</div>
                        )}
                        <button
                          type='button'
                          role='option'
                          aria-selected={i === cursor}
                          onMouseEnter={() => setCursor(i)}
                          onClick={() => run(command)}
                          className={cn(
                            'flex h-8 w-full items-center gap-2.5 px-3.5 text-left text-[13px]',
                            i === cursor
                              ? 'bg-surface-active text-fg'
                              : 'text-fg-muted'
                          )}
                        >
                          {ICONS[command.icon]}
                          <span className={cn('truncate', command.mono && 'font-mono text-xs')}>
                            {command.label}
                          </span>
                          {command.hint && (
                            <span className='ml-auto shrink-0 font-mono text-[11.5px] text-fg-subtle'>
                              {command.hint}
                            </span>
                          )}
                        </button>
                      </Fragment>
                    );
                  })
                )}
              </div>

              <div className='flex gap-3.5 border-t border-border px-3.5 py-2 text-[11.5px] text-fg-subtle'>
                <span className='flex items-center gap-1'>
                  <Kbd>↑</Kbd> <Kbd>↓</Kbd> move
                </span>
                <span className='flex items-center gap-1'>
                  <Kbd>↵</Kbd> open
                </span>
                <span className='flex items-center gap-1'>
                  <Kbd>Esc</Kbd> close
                </span>
              </div>
            </Dialog.Panel>
          </Transition.Child>
        </div>
      </Dialog>
    </Transition.Root>
  );
}
