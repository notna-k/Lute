import { Menu, MenuButton, MenuItem, MenuItems } from '@headlessui/react';
import { LogOut } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';

export interface UserMenuProps {
  /** Avatar only, for the collapsed rail. */
  compact?: boolean;
}

function initialsOf(name: string): string {
  return name
    .split(/[\s@._-]+/)
    .filter(Boolean)
    .map((part) => part[0])
    .slice(0, 2)
    .join('')
    .toUpperCase();
}

export function UserMenu({ compact }: UserMenuProps) {
  const { user, signOut } = useAuth();
  const navigate = useNavigate();

  if (!user) return null;

  const name = user.display_name || user.email;
  const initials = initialsOf(name || 'U');

  async function handleSignOut() {
    try {
      await signOut();
      navigate('/login');
    } catch (error) {
      console.error('Error signing out:', error);
    }
  }

  return (
    <Menu as='div' className='relative flex min-w-0'>
      <MenuButton
        title={name}
        className='flex min-w-0 items-center gap-2.5 text-[12.5px] text-fg-muted hover:text-fg focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg'
      >
        <span className='grid h-6 w-6 shrink-0 place-items-center bg-surface-active font-mono text-[10px] font-semibold text-fg'>
          {initials}
        </span>
        {!compact && <span className='truncate'>{name}</span>}
      </MenuButton>
      <MenuItems
        transition
        className='absolute bottom-full left-0 z-50 mb-2 w-56 border border-border bg-surface py-1 shadow-popover focus:outline-none transition duration-100 ease-out data-[closed]:translate-y-1 data-[closed]:opacity-0 data-[leave]:duration-75 data-[leave]:ease-in'
      >
        <div className='border-b border-border px-3 py-2'>
          <div className='truncate text-[13px] font-medium text-fg'>
            {user.display_name || 'User'}
          </div>
          <div className='truncate font-mono text-[11px] text-fg-muted'>{user.email}</div>
        </div>
        <MenuItem>
          <button
            type='button'
            onClick={handleSignOut}
            className='flex w-full items-center gap-2 px-3 py-2 text-[13px] text-fg data-[focus]:bg-surface-hover'
          >
            <LogOut className='h-3.5 w-3.5' />
            Sign out
          </button>
        </MenuItem>
      </MenuItems>
    </Menu>
  );
}
