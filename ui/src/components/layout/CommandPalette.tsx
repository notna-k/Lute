import { Fragment, useEffect, useMemo, useState } from 'react';
import { Dialog, Transition } from '@headlessui/react';
import { useNavigate } from 'react-router-dom';
import { useQuery } from '@tanstack/react-query';
import { CornerDownLeft, Play, Search } from 'lucide-react';
import { listJobs } from '@/services/jobDefService';
import { cn } from '@/lib/cn';
import { NAV_ITEMS } from './nav';

interface Command {
  id: string;
  label: string;
  hint: string;
  to: string;
  kind: 'page' | 'job';
}

interface CommandPaletteProps {
  open: boolean;
  onClose: () => void;
}

/**
 * ⌘K launcher: jump to a page or straight to a job's trigger form. Backs the
 * command affordance the rail and top bar advertise.
 */
export function CommandPalette({ open, onClose }: CommandPaletteProps) {
  const navigate = useNavigate();
  const [query, setQuery] = useState('');
  const [cursor, setCursor] = useState(0);

  // Only fetch the job list while the palette is actually open.
  const { data: jobs } = useQuery({
    queryKey: ['jobs'],
    queryFn: listJobs,
    enabled: open,
  });

  const commands = useMemo<Command[]>(() => {
    const pages: Command[] = NAV_ITEMS.map((i) => ({
      id: `page:${i.to}`,
      label: i.label,
      hint: 'Go to',
      to: i.to,
      kind: 'page',
    }));
    const jobCommands: Command[] = (jobs ?? []).map((j) => ({
      id: `job:${j.slug}`,
      label: j.name,
      hint: `Run · ${j.queue}`,
      to: `/jobs/${j.slug}`,
      kind: 'job',
    }));
    return [...pages, ...jobCommands];
  }, [jobs]);

  const results = useMemo(() => {
    const q = query.trim().toLowerCase();
    if (!q) return commands;
    return commands.filter((c) => c.label.toLowerCase().includes(q));
  }, [commands, query]);

  useEffect(() => {
    setCursor(0);
  }, [query, open]);

  useEffect(() => {
    if (!open) setQuery('');
  }, [open]);

  function run(command: Command | undefined) {
    if (!command) return;
    onClose();
    navigate(command.to);
  }

  function onKeyDown(e: React.KeyboardEvent) {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      setCursor((c) => Math.min(c + 1, results.length - 1));
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      setCursor((c) => Math.max(c - 1, 0));
    } else if (e.key === 'Enter') {
      e.preventDefault();
      run(results[cursor]);
    }
  }

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
          <div className='fixed inset-0 bg-black/60 backdrop-blur-sm' />
        </Transition.Child>

        <div className='fixed inset-0 overflow-y-auto p-4 pt-[15vh]'>
          <Transition.Child
            as={Fragment}
            enter='ease-out duration-150'
            enterFrom='opacity-0 scale-95'
            enterTo='opacity-100 scale-100'
            leave='ease-in duration-100'
            leaveFrom='opacity-100 scale-100'
            leaveTo='opacity-0 scale-95'
          >
            <Dialog.Panel className='mx-auto max-w-xl overflow-hidden rounded-xl border border-border bg-bg-elevated shadow-popover'>
              <div className='flex items-center gap-2.5 border-b border-border px-4'>
                <Search className='h-4 w-4 shrink-0 text-fg-subtle' />
                <input
                  autoFocus
                  value={query}
                  onChange={(e) => setQuery(e.target.value)}
                  onKeyDown={onKeyDown}
                  placeholder='Run any job, jump to a page…'
                  className='w-full bg-transparent py-3.5 text-sm text-fg placeholder:text-fg-subtle focus:outline-none'
                />
                <kbd className='shrink-0 rounded border border-b-2 border-border bg-surface px-1.5 py-px font-mono text-[10px] text-fg-muted'>
                  esc
                </kbd>
              </div>

              <div className='scrollbar-thin max-h-80 overflow-auto p-1.5'>
                {results.length === 0 ? (
                  <p className='px-3 py-6 text-center text-sm text-fg-muted'>
                    Nothing matches “{query}”.
                  </p>
                ) : (
                  results.map((c, i) => (
                    <button
                      key={c.id}
                      type='button'
                      onMouseEnter={() => setCursor(i)}
                      onClick={() => run(c)}
                      className={cn(
                        'flex w-full items-center gap-2.5 rounded-lg px-3 py-2 text-left text-sm',
                        i === cursor ? 'bg-surface-hover text-fg' : 'text-fg-muted'
                      )}
                    >
                      {c.kind === 'job' ? (
                        <Play className='h-3.5 w-3.5 text-primary' />
                      ) : (
                        <CornerDownLeft className='h-3.5 w-3.5 text-fg-subtle' />
                      )}
                      <span className='truncate text-fg'>{c.label}</span>
                      <span className='ml-auto shrink-0 font-mono text-[11px] text-fg-subtle'>
                        {c.hint}
                      </span>
                    </button>
                  ))
                )}
              </div>
            </Dialog.Panel>
          </Transition.Child>
        </div>
      </Dialog>
    </Transition.Root>
  );
}
