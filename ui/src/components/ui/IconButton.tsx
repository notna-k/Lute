import { forwardRef, type ButtonHTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

type IconButtonVariant = 'ghost' | 'outline' | 'solid' | 'danger';
type IconButtonSize = 'sm' | 'md';

export interface IconButtonProps extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: IconButtonVariant;
  size?: IconButtonSize;
  /** Required: icon-only controls need an accessible name, and it doubles as the tooltip. */
  label: string;
}

const VARIANTS: Record<IconButtonVariant, string> = {
  ghost: 'bg-transparent text-fg-subtle hover:bg-surface-hover hover:text-fg',
  outline:
    'border border-border bg-transparent text-fg-muted hover:border-border-strong hover:text-fg',
  solid: 'border border-border bg-surface text-fg hover:bg-surface-hover',
  danger: 'bg-transparent text-danger hover:bg-danger/10',
};

const SIZES: Record<IconButtonSize, string> = {
  sm: 'h-[22px] w-[22px]',
  md: 'h-[26px] w-[26px]',
};

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(function IconButton(
  { variant = 'ghost', size = 'md', label, className, type = 'button', children, ...rest },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      aria-label={label}
      title={label}
      className={cn(
        'inline-grid shrink-0 place-items-center transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg focus-visible:ring-offset-1 focus-visible:ring-offset-bg disabled:pointer-events-none disabled:opacity-45',
        VARIANTS[variant],
        SIZES[size],
        className,
      )}
      {...rest}
    >
      {children}
    </button>
  );
});
