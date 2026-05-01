import { Menu, Transition } from '@headlessui/react';
import { Fragment } from 'react';
import { Monitor, Moon, Sun } from 'lucide-react';
import { useTheme, type ThemeMode } from '@/contexts/ThemeContext';
import { cn } from '@/lib/cn';

const OPTIONS: { value: ThemeMode; label: string; icon: typeof Sun }[] = [
  { value: 'light', label: 'Light', icon: Sun },
  { value: 'dark', label: 'Dark', icon: Moon },
  { value: 'system', label: 'System', icon: Monitor },
];

export function ThemeToggle() {
  const { mode, resolved, setMode } = useTheme();
  const Icon = resolved === 'dark' ? Moon : Sun;

  return (
    <Menu as='div' className='relative'>
      <Menu.Button
        className='inline-flex h-9 w-9 items-center justify-center rounded-md border border-border bg-surface text-fg-muted hover:bg-surface-hover hover:text-fg'
        aria-label='Toggle theme'
      >
        <Icon className='h-4 w-4' />
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
        <Menu.Items className='absolute right-0 z-50 mt-2 w-40 origin-top-right rounded-md border border-border bg-surface py-1 shadow-popover focus:outline-none'>
          {OPTIONS.map((opt) => {
            const OptIcon = opt.icon;
            const active = mode === opt.value;
            return (
              <Menu.Item key={opt.value}>
                {({ active: hover }) => (
                  <button
                    type='button'
                    onClick={() => setMode(opt.value)}
                    className={cn(
                      'flex w-full items-center gap-2 px-3 py-2 text-sm',
                      hover && 'bg-surface-hover',
                      active ? 'text-primary' : 'text-fg'
                    )}
                  >
                    <OptIcon className='h-4 w-4' />
                    <span className='flex-1 text-left'>{opt.label}</span>
                    {active && (
                      <span
                        aria-hidden
                        className='h-1.5 w-1.5 rounded-full bg-primary'
                      />
                    )}
                  </button>
                )}
              </Menu.Item>
            );
          })}
        </Menu.Items>
      </Transition>
    </Menu>
  );
}
