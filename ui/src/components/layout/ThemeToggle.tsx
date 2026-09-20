import { Moon, Sun } from 'lucide-react';
import { useTheme } from '@/contexts/ThemeContext';
import { IconButton } from '@/components/ui';

export interface ThemeToggleProps {
  className?: string;
}

/**
 * A straight light/dark flip.
 *
 * The three-way choice (light, dark, follow the system) lives in Settings; in
 * the rail a single click is what an operator wants when the room's light
 * changes, not a menu.
 */
export function ThemeToggle({ className }: ThemeToggleProps) {
  const { resolved, toggle } = useTheme();
  const dark = resolved === 'dark';
  return (
    <IconButton
      label={dark ? 'Switch to the light theme' : 'Switch to the dark theme'}
      onClick={toggle}
      className={className}
    >
      {dark ? <Sun className='h-[15px] w-[15px]' /> : <Moon className='h-[15px] w-[15px]' />}
    </IconButton>
  );
}
