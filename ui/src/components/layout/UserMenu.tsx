import { Fragment } from 'react';
import { Menu, Transition } from '@headlessui/react';
import { LogOut, User as UserIcon } from 'lucide-react';
import { useNavigate } from 'react-router-dom';
import { useAuth } from '@/contexts/AuthContext';
import { cn } from '@/lib/cn';

export function UserMenu() {
  const { user, signOut } = useAuth();
  const navigate = useNavigate();

  if (!user) return null;

  const handleLogout = async () => {
    try {
      await signOut();
      navigate('/login');
    } catch (e) {
      console.error('Error logging out:', e);
    }
  };

  const initials = (user.display_name || user.email || 'U')
    .split(/\s+/)
    .map((s) => s[0])
    .slice(0, 2)
    .join('')
    .toUpperCase();

  return (
    <Menu as='div' className='relative'>
      <Menu.Button className='flex items-center gap-2 rounded-full p-0.5 pr-2 text-sm text-fg hover:bg-surface-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary'>
        <span className='flex h-8 w-8 items-center justify-center rounded-full bg-primary-subtle text-xs font-semibold text-info-fg'>
          {initials}
        </span>
        <span className='hidden max-w-[10rem] truncate sm:inline'>
          {user.display_name || user.email}
        </span>
      </Menu.Button>
      <Transition
        as={Fragment}
        enter='transition ease-out duration-100'
        enterFrom='opacity-0 translate-y-1'
        enterTo='opacity-100 translate-y-0'
        leave='transition ease-in duration-75'
        leaveFrom='opacity-100'
        leaveTo='opacity-0'
      >
        <Menu.Items className='absolute right-0 z-50 mt-2 w-56 origin-top-right rounded-md border border-border bg-surface py-1 shadow-popover focus:outline-none'>
          <div className='border-b border-border px-3 py-2'>
            <div className='truncate text-sm font-medium text-fg'>
              {user.display_name || 'User'}
            </div>
            <div className='truncate text-xs text-fg-muted'>{user.email}</div>
          </div>
          <Menu.Item disabled>
            {() => (
              <span
                className={cn(
                  'flex items-center gap-2 px-3 py-2 text-sm text-fg-muted'
                )}
              >
                <UserIcon className='h-4 w-4' />
                <span>{user.email}</span>
              </span>
            )}
          </Menu.Item>
          <Menu.Item>
            {({ active }) => (
              <button
                type='button'
                onClick={handleLogout}
                className={cn(
                  'flex w-full items-center gap-2 px-3 py-2 text-sm text-fg',
                  active && 'bg-surface-hover'
                )}
              >
                <LogOut className='h-4 w-4' />
                Sign out
              </button>
            )}
          </Menu.Item>
        </Menu.Items>
      </Transition>
    </Menu>
  );
}
