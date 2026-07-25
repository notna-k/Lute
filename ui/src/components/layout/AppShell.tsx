import { Fragment, useEffect, useState, type ReactNode } from 'react';
import { Dialog, Transition } from '@headlessui/react';
import { useNavigate } from 'react-router-dom';
import { X } from 'lucide-react';
import { Brand, Rail, RailFooter, RailNav } from './Rail';
import { TopBar } from './TopBar';
import { CommandPalette } from './CommandPalette';
import { NAV_ITEMS } from './nav';

/** True when the keystroke belongs to whatever the user is typing into. */
function isTypingTarget(target: EventTarget | null): boolean {
  const el = target as HTMLElement | null;
  if (!el) return false;
  return (
    el.isContentEditable ||
    el.tagName === 'INPUT' ||
    el.tagName === 'TEXTAREA' ||
    el.tagName === 'SELECT'
  );
}

/**
 * Global keyboard shortcuts: ⌘K / Ctrl+K opens the command palette, and the
 * single letters shown in the rail jump to their section.
 */
function useHotkeys(openCommand: () => void) {
  const navigate = useNavigate();
  useEffect(() => {
    function onKeyDown(e: KeyboardEvent) {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        openCommand();
        return;
      }
      if (e.metaKey || e.ctrlKey || e.altKey || isTypingTarget(e.target)) return;
      const item = NAV_ITEMS.find((i) => i.hotkey === e.key.toLowerCase());
      if (item) {
        e.preventDefault();
        navigate(item.to);
      }
    }
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [navigate, openCommand]);
}

interface AppShellProps {
  children: ReactNode;
}

/**
 * Console shell (see poc/console.html): a fixed left rail, a breadcrumb top bar,
 * and the page body. Below `lg` the rail collapses into a drawer.
 */
export function AppShell({ children }: AppShellProps) {
  const [navOpen, setNavOpen] = useState(false);
  const [commandOpen, setCommandOpen] = useState(false);
  useHotkeys(() => setCommandOpen(true));

  return (
    <div className='min-h-screen bg-bg lg:grid lg:grid-cols-[232px_minmax(0,1fr)]'>
      <Rail />

      <main className='flex min-w-0 flex-col'>
        <TopBar
          onOpenNav={() => setNavOpen(true)}
          onOpenCommand={() => setCommandOpen(true)}
        />
        <div className='flex-1 px-4 py-6 sm:px-6'>{children}</div>
      </main>

      <CommandPalette open={commandOpen} onClose={() => setCommandOpen(false)} />

      <Transition.Root show={navOpen} as={Fragment}>
        <Dialog as='div' className='relative z-50 lg:hidden' onClose={setNavOpen}>
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
              <Dialog.Panel className='relative flex w-full max-w-[232px] flex-col gap-5 border-r border-border bg-bg px-3.5 py-4'>
                <div className='flex items-center justify-between'>
                  <Brand />
                  <button
                    type='button'
                    onClick={() => setNavOpen(false)}
                    className='inline-flex h-8 w-8 items-center justify-center rounded-md text-fg-muted hover:bg-surface-hover hover:text-fg'
                    aria-label='Close navigation'
                  >
                    <X className='h-5 w-5' />
                  </button>
                </div>
                <RailNav onNavigate={() => setNavOpen(false)} />
                <RailFooter />
              </Dialog.Panel>
            </Transition.Child>
          </div>
        </Dialog>
      </Transition.Root>
    </div>
  );
}
