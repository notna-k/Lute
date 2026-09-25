import { type HTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

export type BadgeTone = 'neutral' | 'primary' | 'success' | 'warning' | 'danger' | 'info';

type BadgeSize = 'sm' | 'md';

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: BadgeTone;
  size?: BadgeSize;
  dot?: boolean;
}

// For build and worker state use StatusBadge; this tag is for everything else (env, queue, origin).
const TONE_STYLES: Record<BadgeTone, string> = {
  neutral: 'border-border bg-bg-subtle text-fg-muted',
  primary: 'border-border bg-bg-subtle text-fg',
  success: 'border-success/30 bg-success-subtle text-success',
  warning: 'border-warning/30 bg-warning-subtle text-warning',
  danger: 'border-danger/30 bg-danger-subtle text-danger',
  info: 'border-border bg-bg-subtle text-fg-muted',
};

const TONE_DOT: Record<BadgeTone, string> = {
  neutral: 'bg-fg-subtle',
  primary: 'bg-fg',
  success: 'bg-success',
  warning: 'bg-warning',
  danger: 'bg-danger',
  info: 'bg-fg-subtle',
};

const SIZES: Record<BadgeSize, string> = {
  sm: 'text-[11px] px-1.5 h-[18px]',
  md: 'text-xs px-2 h-[22px]',
};

export function Badge({
  tone = 'neutral',
  size = 'md',
  dot,
  className,
  children,
  ...rest
}: BadgeProps) {
  return (
    <span
      className={cn(
        'inline-flex items-center gap-1.5 border font-medium',
        TONE_STYLES[tone],
        SIZES[size],
        className,
      )}
      {...rest}
    >
      {dot && <span aria-hidden className={cn('h-[5px] w-[5px]', TONE_DOT[tone])} />}
      {children}
    </span>
  );
}
