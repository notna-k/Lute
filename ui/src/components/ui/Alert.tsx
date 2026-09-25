import { type HTMLAttributes, type ReactNode } from 'react';
import { AlertTriangle, GitBranch, Info } from 'lucide-react';
import { cn } from '@/lib/cn';

type AlertTone = 'info' | 'success' | 'warning' | 'danger';

// Tone lives in a 2px left edge and the icon, so a banner does not compete with status colours.
const EDGE: Record<AlertTone, string> = {
  info: 'border-l-border-strong',
  success: 'border-l-success',
  warning: 'border-l-warning',
  danger: 'border-l-danger',
};

const ICON_TONE: Record<AlertTone, string> = {
  info: 'text-fg-subtle',
  success: 'text-success',
  warning: 'text-warning',
  danger: 'text-danger',
};

const TONE_ICONS: Record<AlertTone, typeof Info> = {
  info: Info,
  success: GitBranch,
  warning: AlertTriangle,
  danger: AlertTriangle,
};

export interface AlertProps extends Omit<HTMLAttributes<HTMLDivElement>, 'title'> {
  tone?: AlertTone;
  title?: ReactNode;
  /** Buttons that act on what the banner reports, aligned to the right. */
  action?: ReactNode;
  icon?: ReactNode;
}

export function Alert({
  tone = 'info',
  title,
  action,
  icon,
  className,
  children,
  ...rest
}: AlertProps) {
  const Icon = TONE_ICONS[tone];
  return (
    <div
      role={tone === 'danger' ? 'alert' : 'status'}
      className={cn(
        'flex flex-wrap items-center gap-2.5 border border-l-2 border-border bg-surface px-3 py-2.5 text-[13px]',
        EDGE[tone],
        className,
      )}
      {...rest}
    >
      <span className={cn('shrink-0', ICON_TONE[tone])}>
        {icon ?? <Icon className='h-3.5 w-3.5' />}
      </span>
      <div className='min-w-0 flex-[1_1_280px]'>
        {title && <b className='font-medium'>{title}</b>}
        {title && children ? ' ' : null}
        {children && <span className='text-fg-muted'>{children}</span>}
      </div>
      {action && <div className='ml-auto flex shrink-0 gap-1.5'>{action}</div>}
    </div>
  );
}
