import { Link, NavLink, useLocation } from 'react-router-dom';
import { useUserWorkers } from '@/hooks/useWorkers';
import { cn } from '@/lib/cn';
import { DOT_TONE, NAV_GROUPS } from './nav';

const ONLINE = new Set(['running', 'alive']);

export function Brand() {
  return (
    <Link to='/' className='flex items-center gap-2.5'>
      <span className='h-[26px] w-[26px] rounded-[7px] bg-gradient-to-br from-primary to-danger shadow-[0_6px_18px_-6px_rgb(var(--color-danger)/0.6)] ring-1 ring-primary/35' />
      <span className='leading-tight'>
        <b className='block text-[15px] font-bold tracking-[0.02em] text-fg'>
          Lute
        </b>
        <span className='block font-mono text-[10px] uppercase tracking-[0.16em] text-fg-subtle'>
          orchestrator
        </span>
      </span>
    </Link>
  );
}

interface RailNavProps {
  onNavigate?: () => void;
}

/** The grouped link list — shared by the fixed rail and the mobile drawer. */
export function RailNav({ onNavigate }: RailNavProps) {
  const { pathname } = useLocation();
  return (
    <div className='flex flex-col gap-3.5'>
      {NAV_GROUPS.map((group) => (
        <div key={group.label}>
          <div className='px-2 pb-1.5 font-mono text-[10px] uppercase tracking-[0.18em] text-fg-subtle'>
            {group.label}
          </div>
          <nav className='flex flex-col gap-0.5'>
            {group.items.map((item) => {
              const active = item.match(pathname);
              return (
                <NavLink
                  key={item.to}
                  to={item.to}
                  onClick={onNavigate}
                  className={cn(
                    'flex items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-[13px] font-medium transition-colors',
                    active
                      ? 'bg-surface-active text-fg shadow-[inset_2px_0_0_rgb(var(--color-primary))]'
                      : 'text-fg-muted hover:bg-surface hover:text-fg'
                  )}
                >
                  <span
                    className={cn('h-1.5 w-1.5 rounded-full', DOT_TONE[item.tone])}
                  />
                  {item.label}
                  {item.hotkey && (
                    <span className='ml-auto font-mono text-[11px] uppercase text-fg-subtle'>
                      {item.hotkey}
                    </span>
                  )}
                </NavLink>
              );
            })}
          </nav>
        </div>
      ))}
    </div>
  );
}

/** Live status line: how many registered workers are currently reachable. */
function RailStatus() {
  const { data: workers } = useUserWorkers();
  if (!workers) return null;
  const up = workers.filter((w) => ONLINE.has(w.status)).length;
  return (
    <div>
      {up} of {workers.length} worker{workers.length === 1 ? '' : 's'} up
    </div>
  );
}

export function RailFooter() {
  return (
    <div className='mt-auto flex flex-col gap-1.5 font-mono text-[11px] text-fg-subtle'>
      <div className='flex items-center gap-1.5'>
        <kbd className='rounded border border-b-2 border-border bg-surface px-1.5 py-px text-[10px] text-fg-muted'>
          ⌘
        </kbd>
        <kbd className='rounded border border-b-2 border-border bg-surface px-1.5 py-px text-[10px] text-fg-muted'>
          K
        </kbd>
        <span className='ml-1'>command</span>
      </div>
      <RailStatus />
    </div>
  );
}

/** Fixed sidebar. Hidden below the lg breakpoint, where the drawer takes over. */
export function Rail() {
  return (
    <aside className='sticky top-0 hidden h-screen flex-col gap-5 border-r border-border px-3.5 py-4 lg:flex'>
      <Brand />
      <RailNav />
      <RailFooter />
    </aside>
  );
}
