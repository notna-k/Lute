import { Menu as MenuIcon } from 'lucide-react';
import { useLocation } from 'react-router-dom';
import { ThemeToggle } from './ThemeToggle';
import { UserMenu } from './UserMenu';
import { breadcrumbFor } from './nav';

interface TopBarProps {
  onOpenNav: () => void;
  onOpenCommand: () => void;
}

export function TopBar({ onOpenNav, onOpenCommand }: TopBarProps) {
  const { pathname } = useLocation();
  const crumbs = breadcrumbFor(pathname);

  return (
    <div className='sticky top-0 z-30 flex items-center gap-3.5 border-b border-border bg-bg/80 px-4 py-2.5 backdrop-blur sm:px-6'>
      <button
        type='button'
        onClick={onOpenNav}
        className='-ml-1 inline-flex h-8 w-8 items-center justify-center rounded-md text-fg-muted hover:bg-surface-hover hover:text-fg lg:hidden'
        aria-label='Open navigation'
      >
        <MenuIcon className='h-5 w-5' />
      </button>

      <div className='min-w-0 truncate font-mono text-xs text-fg-muted'>
        {crumbs.map((crumb, i) => (
          <span key={`${crumb}-${i}`}>
            {i > 0 && <span className='mx-1.5 text-fg-subtle'>/</span>}
            <span className={i === crumbs.length - 1 ? 'text-fg' : undefined}>
              {crumb}
            </span>
          </span>
        ))}
      </div>

      <button
        type='button'
        onClick={onOpenCommand}
        className='ml-auto hidden items-center gap-2 rounded-lg border border-border bg-surface px-2.5 py-1.5 text-xs text-fg-muted hover:border-border-strong hover:text-fg sm:inline-flex'
      >
        <kbd className='rounded border border-b-2 border-border bg-bg px-1.5 py-px font-mono text-[10px]'>
          ⌘
        </kbd>
        <kbd className='rounded border border-b-2 border-border bg-bg px-1.5 py-px font-mono text-[10px]'>
          K
        </kbd>
        Run any job…
      </button>

      <div className='ml-auto flex items-center gap-2 sm:ml-0'>
        <ThemeToggle />
        <UserMenu />
      </div>
    </div>
  );
}
