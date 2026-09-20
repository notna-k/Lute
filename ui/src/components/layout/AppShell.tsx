import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { CommandPalette } from './CommandPalette';
import { Sidebar } from './Sidebar';
import { NAV_ITEMS } from './nav';
import { useUiPreferences } from '@/contexts/UiPreferencesContext';

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
 * Global keys: the palette on Ctrl/⌘ K or `/`, the rail width on `[`, and one
 * letter per nav entry. Bare keys are ignored while the user is typing.
 */
function useHotkeys(openCommand: () => void, toggleSidebar: () => void) {
  const navigate = useNavigate();
  useEffect(() => {
    function onKeyDown(event: KeyboardEvent) {
      const mod = event.metaKey || event.ctrlKey;
      if (mod && event.key.toLowerCase() === 'k') {
        event.preventDefault();
        openCommand();
        return;
      }
      if (mod || event.altKey || isTypingTarget(event.target)) return;
      if (event.key === '/') {
        event.preventDefault();
        openCommand();
        return;
      }
      if (event.key === '[') {
        event.preventDefault();
        toggleSidebar();
        return;
      }
      const item = NAV_ITEMS.find((i) => i.hotkey === event.key.toLowerCase());
      if (item) {
        event.preventDefault();
        navigate(item.to);
      }
    }
    document.addEventListener('keydown', onKeyDown);
    return () => document.removeEventListener('keydown', onKeyDown);
  }, [navigate, openCommand, toggleSidebar]);
}

export interface AppShellProps {
  children: ReactNode;
}

/**
 * The application frame: a collapsible rail and one full-height page column.
 *
 * Nothing here scrolls. Each page owns its own scrolling regions, which is what
 * lets a job's header and tabs stay fixed while its build log streams, and lets
 * the build list and the log scroll past each other independently.
 */
export function AppShell({ children }: AppShellProps) {
  const { sidebarExpanded, toggleSidebar } = useUiPreferences();
  const [commandOpen, setCommandOpen] = useState(false);
  const openCommand = useCallback(() => setCommandOpen(true), []);
  useHotkeys(openCommand, toggleSidebar);

  return (
    <div className='group/sidebar flex h-full min-h-0 max-md:block max-md:h-auto max-md:min-h-screen'>
      <Sidebar expanded={sidebarExpanded} onToggle={toggleSidebar} onOpenCommand={openCommand} />
      <main className='flex min-h-0 min-w-0 flex-1 flex-col overflow-hidden max-md:overflow-visible'>
        {children}
      </main>
      <CommandPalette open={commandOpen} onClose={() => setCommandOpen(false)} />
    </div>
  );
}
