import {
  LayoutDashboard,
  Server,
  Settings,
  Terminal,
  Workflow,
  type LucideIcon,
} from 'lucide-react';

export interface NavItem {
  to: string;
  label: string;
  icon: LucideIcon;
  /** Colour of the rail's leading dot. */
  tone: 'signal' | 'teal' | 'violet' | 'muted';
  /** Single-key shortcut, shown in the rail and bound globally (see useHotkeys). */
  hotkey?: string;
  match: (pathname: string) => boolean;
}

export interface NavGroup {
  label: string;
  items: NavItem[];
}

/**
 * The rail's navigation, grouped as in poc/console.html. Only routes that
 * actually exist are listed — the POC's aspirational entries (Queue, Git
 * sources, Secrets) are left out until they have pages.
 */
export const NAV_GROUPS: NavGroup[] = [
  {
    label: 'Operate',
    items: [
      {
        to: '/',
        label: 'Dashboard',
        icon: LayoutDashboard,
        tone: 'muted',
        hotkey: 'd',
        match: (p) => p === '/' || p === '/dashboard',
      },
      {
        to: '/jobs',
        label: 'Jobs',
        icon: Workflow,
        tone: 'signal',
        hotkey: 'j',
        match: (p) => p.startsWith('/jobs'),
      },
      {
        // Runs are "builds" in PRODUCT.md's model; the route stays /executions.
        to: '/executions',
        label: 'Builds',
        icon: Terminal,
        tone: 'teal',
        hotkey: 'b',
        match: (p) => p.startsWith('/executions'),
      },
      {
        to: '/workers',
        label: 'Workers',
        icon: Server,
        tone: 'violet',
        hotkey: 'w',
        match: (p) => p.startsWith('/workers'),
      },
    ],
  },
  {
    label: 'System',
    items: [
      {
        to: '/settings',
        label: 'Settings',
        icon: Settings,
        tone: 'muted',
        match: (p) => p.startsWith('/settings'),
      },
    ],
  },
];

export const NAV_ITEMS: NavItem[] = NAV_GROUPS.flatMap((g) => g.items);

export const DOT_TONE: Record<NavItem['tone'], string> = {
  signal: 'bg-primary',
  teal: 'bg-success',
  violet: 'bg-info',
  muted: 'bg-fg-subtle',
};

/** Breadcrumb trail for a pathname, e.g. /jobs/web-release → Jobs · web-release. */
export function breadcrumbFor(pathname: string): string[] {
  const segments = pathname.split('/').filter(Boolean);
  if (segments.length === 0) return ['Dashboard'];
  const root = NAV_ITEMS.find((i) => i.match(pathname));
  const trail = [root?.label ?? segments[0]];
  return trail.concat(segments.slice(1));
}
