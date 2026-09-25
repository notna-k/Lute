import { useCallback, useEffect, useState, type ReactNode } from 'react';
import { useNavigate } from 'react-router-dom';
import { CommandPalette } from './CommandPalette';
import { Sidebar } from './Sidebar';
import { NAV_ITEMS } from './nav';
import { useUiPreferences } from '@/contexts/UiPreferencesContext';
import { isTypingTarget } from '@/lib/dom';

/** Global keys: palette on Ctrl/⌘K or `/`, rail on `[`, one letter per nav entry; ignored while typing. */
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

/** The app frame. Nothing here scrolls: each page owns its scroll regions, so headers stay put. */
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
