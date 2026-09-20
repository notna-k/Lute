import { Link, NavLink, useLocation } from 'react-router-dom';
import { PanelLeftClose, PanelLeftOpen, Search } from 'lucide-react';
import { useQuery } from '@tanstack/react-query';
import { listJobs } from '@/services/jobDefService';
import { useUserWorkers } from '@/hooks/useWorkers';
import { IconButton, Kbd, MOD_KEY } from '@/components/ui';
import { cn } from '@/lib/cn';
import { NAV_ITEMS } from './nav';
import { ThemeToggle } from './ThemeToggle';
import { UserMenu } from './UserMenu';

const ONLINE_STATUSES = new Set(['running', 'alive']);

/** Live trailing counts, so the rail doubles as a status line. */
function useNavCounts(): Record<string, string | undefined> {
  const { data: jobs } = useQuery({ queryKey: ['jobs'], queryFn: listJobs });
  const { data: workers } = useUserWorkers();
  const online = workers?.filter((w) => ONLINE_STATUSES.has(w.status)).length;
  return {
    '/jobs': jobs ? String(jobs.length) : undefined,
    '/workers': workers ? `${online}/${workers.length}` : undefined,
  };
}

export interface SidebarProps {
  /** Icons only, or icons with labels and counts. Persisted in Settings. */
  expanded: boolean;
  onToggle: () => void;
  onOpenCommand: () => void;
  /** Called after a link is followed, so the mobile drawer can close itself. */
  onNavigate?: () => void;
  className?: string;
}

/**
 * The one piece of permanent navigation.
 *
 * It collapses to a 56px icon rail because the pages it leads to are wide —
 * a build log next to its step list needs the horizontal space more than the
 * nav needs its labels. Collapsed, every target keeps a tooltip and the counts
 * survive as a marker dot, so nothing is only available when expanded.
 */
export function Sidebar({
  expanded,
  onToggle,
  onOpenCommand,
  onNavigate,
  className,
}: SidebarProps) {
  const { pathname } = useLocation();
  const counts = useNavCounts();

  return (
    <nav
      className={cn(
        'relative flex w-14 shrink-0 flex-col gap-0.5 border-r border-border bg-bg px-2 pb-2.5 pt-2 transition-[width] duration-150',
        expanded && 'w-[216px]',
        // On phones the rail lies down as a top bar.
        'max-md:w-auto max-md:flex-row max-md:flex-wrap max-md:items-center max-md:border-b max-md:border-r-0 max-md:px-3 max-md:py-1.5',
        className
      )}
    >
      <div className='flex h-10 items-center gap-2.5 pl-0.5'>
        <Link
          to='/'
          aria-label='Lute'
          className='grid h-6 w-6 shrink-0 place-items-center border border-fg bg-fg text-[12px] font-semibold text-bg'
        >
          L
        </Link>
        {expanded && (
          <span className='min-w-0 flex-1 truncate text-[13.5px] max-md:hidden'>
            <b className='font-semibold'>Lute</b>{' '}
            <span className='text-fg-subtle'>panel</span>
          </span>
        )}
        <IconButton
          label={expanded ? 'Collapse menu — [' : 'Expand menu — ['}
          onClick={onToggle}
          className={cn(
            'max-md:hidden',
            expanded
              ? 'ml-auto'
              : // Collapsed, the toggle would crowd the logo, so it surfaces on
                // hover as a floating affordance instead.
                'absolute left-3 top-[54px] z-[3] h-6 w-6 border border-border bg-surface opacity-0 transition-opacity focus-visible:opacity-100 group-hover/sidebar:opacity-100'
          )}
        >
          {expanded ? (
            <PanelLeftClose className='h-[15px] w-[15px]' />
          ) : (
            <PanelLeftOpen className='h-[15px] w-[15px]' />
          )}
        </IconButton>
      </div>

      <button
        type='button'
        onClick={onOpenCommand}
        title={`Search — ${MOD_KEY} K`}
        className={cn(
          'mb-1.5 flex h-[34px] items-center gap-2.5 border border-transparent px-2.5 text-fg-subtle transition-colors hover:bg-surface-hover hover:text-fg max-md:hidden',
          expanded && 'border-border bg-surface'
        )}
      >
        <Search className='h-4 w-4 shrink-0' />
        {expanded && (
          <>
            <span className='text-[13px]'>Search</span>
            <span className='ml-auto'>
              <Kbd>{MOD_KEY} K</Kbd>
            </span>
          </>
        )}
      </button>

      <div className='grid gap-px max-md:grid-flow-col'>
        {NAV_ITEMS.map((item) => {
          const active = item.match(pathname);
          const count = counts[item.to];
          return (
            <NavLink
              key={item.to}
              to={item.to}
              onClick={onNavigate}
              aria-current={active ? 'page' : undefined}
              title={expanded ? undefined : item.label}
              className={cn(
                'relative flex h-[34px] items-center gap-3 px-2.5 text-fg-muted transition-colors hover:bg-surface-hover hover:text-fg',
                active &&
                  'bg-surface-active text-fg shadow-[inset_2px_0_0_rgb(var(--color-fg))]'
              )}
            >
              <item.icon className='h-4 w-4 shrink-0' />
              {expanded && (
                <>
                  <span className='text-[13px]'>{item.label}</span>
                  {count && (
                    <span className='ml-auto font-mono text-[11px] text-fg-subtle tabular-nums'>
                      {count}
                    </span>
                  )}
                </>
              )}
            </NavLink>
          );
        })}
      </div>

      <span className='flex-1' />

      <div
        className={cn(
          'flex items-center gap-2.5 border-t border-border pt-2 text-[12.5px] text-fg-muted max-md:ml-auto max-md:border-0 max-md:pt-0',
          !expanded && 'flex-col-reverse gap-1.5 px-0 max-md:flex-row'
        )}
      >
        <UserMenu compact={!expanded} />
        <ThemeToggle className={cn(expanded && 'ml-auto')} />
      </div>
    </nav>
  );
}
