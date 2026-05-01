import { Fragment, useState } from 'react';
import { Dialog, Transition } from '@headlessui/react';
import { Link, NavLink, useLocation } from 'react-router-dom';
import {
  LayoutDashboard,
  Menu as MenuIcon,
  Server,
  Settings,
  Terminal,
  X,
} from 'lucide-react';
import { useAuth } from '@/contexts/AuthContext';
import { cn } from '@/lib/cn';
import { ThemeToggle } from './ThemeToggle';
import { UserMenu } from './UserMenu';

interface NavItem {
  to: string;
  label: string;
  icon: typeof LayoutDashboard;
  match?: (pathname: string) => boolean;
}

const NAV_ITEMS: NavItem[] = [
  { to: '/', label: 'Dashboard', icon: LayoutDashboard, match: (p) => p === '/' || p === '/dashboard' },
  { to: '/workers', label: 'Workers', icon: Server, match: (p) => p.startsWith('/workers') },
  { to: '/executions', label: 'Executions', icon: Terminal, match: (p) => p.startsWith('/executions') || p.startsWith('/jobs') },
  { to: '/settings', label: 'Settings', icon: Settings, match: (p) => p.startsWith('/settings') },
];

function isActive(pathname: string, item: NavItem): boolean {
  if (item.match) return item.match(pathname);
  return pathname.startsWith(item.to);
}

interface NavLinksProps {
  orientation?: 'horizontal' | 'vertical';
  onNavigate?: () => void;
}

function NavLinks({ orientation = 'horizontal', onNavigate }: NavLinksProps) {
  const { pathname } = useLocation();
  const isVertical = orientation === 'vertical';
  return (
    <nav
      className={cn(
        'flex gap-1',
        isVertical ? 'flex-col' : 'flex-row items-center'
      )}
    >
      {NAV_ITEMS.map((item) => {
        const Icon = item.icon;
        const active = isActive(pathname, item);
        return (
          <NavLink
            key={item.to}
            to={item.to}
            onClick={onNavigate}
            className={cn(
              'inline-flex items-center gap-2 rounded-md px-3 py-2 text-sm font-medium transition-colors',
              active
                ? 'bg-primary-subtle text-info-fg'
                : 'text-fg-muted hover:bg-surface-hover hover:text-fg',
              isVertical && 'w-full'
            )}
          >
            <Icon className='h-4 w-4' />
            <span>{item.label}</span>
          </NavLink>
        );
      })}
    </nav>
  );
}

function Brand() {
  return (
    <Link
      to='/'
      className='flex items-center gap-2 text-lg font-bold tracking-tight text-fg'
    >
      <span className='flex h-7 w-7 items-center justify-center rounded-md bg-primary text-fg-onPrimary'>
        <Terminal className='h-4 w-4' />
      </span>
      <span>Lute</span>
    </Link>
  );
}

export function Navbar() {
  const { user } = useAuth();
  const [mobileOpen, setMobileOpen] = useState(false);

  return (
    <>
      <header className='sticky top-0 z-40 border-b border-border bg-bg/80 backdrop-blur'>
        <div className='mx-auto flex h-14 max-w-7xl items-center gap-4 px-4 sm:px-6'>
          <button
            type='button'
            onClick={() => setMobileOpen(true)}
            className='inline-flex h-9 w-9 items-center justify-center rounded-md text-fg-muted hover:bg-surface-hover hover:text-fg lg:hidden'
            aria-label='Open navigation'
          >
            <MenuIcon className='h-5 w-5' />
          </button>

          <Brand />

          {user && (
            <div className='hidden lg:ml-4 lg:flex'>
              <NavLinks />
            </div>
          )}

          <div className='ml-auto flex items-center gap-2'>
            <ThemeToggle />
            {user && <UserMenu />}
          </div>
        </div>
      </header>

      <Transition.Root show={mobileOpen} as={Fragment}>
        <Dialog
          as='div'
          className='relative z-50 lg:hidden'
          onClose={setMobileOpen}
        >
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
          <div className='fixed inset-0 flex'>
            <Transition.Child
              as={Fragment}
              enter='transition ease-out duration-150'
              enterFrom='-translate-x-full'
              enterTo='translate-x-0'
              leave='transition ease-in duration-100'
              leaveFrom='translate-x-0'
              leaveTo='-translate-x-full'
            >
              <Dialog.Panel className='relative flex w-full max-w-xs flex-col border-r border-border bg-surface py-4'>
                <div className='flex items-center justify-between px-4'>
                  <Brand />
                  <button
                    type='button'
                    onClick={() => setMobileOpen(false)}
                    className='inline-flex h-9 w-9 items-center justify-center rounded-md text-fg-muted hover:bg-surface-hover hover:text-fg'
                    aria-label='Close navigation'
                  >
                    <X className='h-5 w-5' />
                  </button>
                </div>
                <div className='mt-6 px-3'>
                  <NavLinks
                    orientation='vertical'
                    onNavigate={() => setMobileOpen(false)}
                  />
                </div>
              </Dialog.Panel>
            </Transition.Child>
          </div>
        </Dialog>
      </Transition.Root>
    </>
  );
}
