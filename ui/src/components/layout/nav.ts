import {
  Activity,
  ListTree,
  Server,
  Settings,
  SquareChevronRight,
  type LucideIcon,
} from 'lucide-react';

export interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  /** Single-key shortcut, bound globally (see useHotkeys) and shown in the rail. */
  hotkey?: string;
  match: (pathname: string) => boolean;
}

/**
 * The sidebar, in the order an operator works: what is happening, what can be
 * run, what has run, what it runs on, and then configuration.
 */
export const NAV_ITEMS: NavItem[] = [
  {
    to: '/',
    label: 'Overview',
    icon: Activity,
    hotkey: 'd',
    match: (p) => p === '/' || p === '/dashboard',
  },
  {
    to: '/jobs',
    label: 'Jobs',
    icon: ListTree,
    hotkey: 'j',
    match: (p) => p.startsWith('/jobs'),
  },
  {
    // Runs are "builds" in the product model; the route stays /executions.
    to: '/executions',
    label: 'Builds',
    icon: SquareChevronRight,
    hotkey: 'b',
    match: (p) => p.startsWith('/executions'),
  },
  {
    to: '/workers',
    label: 'Workers',
    icon: Server,
    hotkey: 'w',
    match: (p) => p.startsWith('/workers'),
  },
  {
    to: '/settings',
    label: 'Settings',
    icon: Settings,
    match: (p) => p.startsWith('/settings'),
  },
];
