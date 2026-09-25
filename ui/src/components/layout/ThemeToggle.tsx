import { Moon, Sun } from 'lucide-react';
import { useTheme } from '@/contexts/ThemeContext';
import { IconButton } from '@/components/ui/IconButton';

export interface ThemeToggleProps {
  className?: string;
}

/** A straight light/dark flip; the follow-system option lives in Settings. */
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
