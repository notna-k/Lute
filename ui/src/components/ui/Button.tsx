import { forwardRef, type ButtonHTMLAttributes, type ReactNode } from 'react';
import { Link, type LinkProps } from 'react-router-dom';
import { Loader2 } from 'lucide-react';
import { cn } from '@/lib/cn';

type ButtonVariant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'danger' | 'link';

type ButtonSize = 'xs' | 'sm' | 'md' | 'lg' | 'icon';

/**
 * Square, hairline-bordered control. `primary` is the inverted fill — there is
 * no brand hue in this palette, so emphasis comes from contrast, and colour is
 * left to mean build status.
 */
const BASE =
  'inline-flex shrink-0 items-center justify-center gap-[7px] whitespace-nowrap font-medium transition-colors focus-visible:outline-none focus-visible:ring-1 focus-visible:ring-fg focus-visible:ring-offset-1 focus-visible:ring-offset-bg disabled:pointer-events-none disabled:opacity-45';

const VARIANTS: Record<ButtonVariant, string> = {
  primary: 'border border-primary bg-primary text-fg-onPrimary hover:opacity-[0.88]',
  secondary:
    'border border-border bg-surface text-fg hover:border-border-strong hover:bg-surface-hover',
  outline:
    'border border-border bg-transparent text-fg hover:border-border-strong hover:bg-surface-hover',
  ghost:
    'border border-transparent bg-transparent text-fg-muted hover:bg-surface-hover hover:text-fg',
  danger: 'border border-border bg-surface text-danger hover:border-danger hover:bg-surface-hover',
  link: 'bg-transparent text-fg underline-offset-[3px] hover:underline p-0 h-auto',
};

const SIZES: Record<ButtonSize, string> = {
  xs: 'h-6 px-2 text-xs gap-1.5',
  sm: 'h-7 px-2.5 text-xs',
  md: 'h-[30px] px-3 text-[13px]',
  lg: 'h-9 px-4 text-sm',
  icon: 'h-[30px] w-[30px]',
};

interface CommonProps {
  variant?: ButtonVariant;
  size?: ButtonSize;
  leftIcon?: ReactNode;
  rightIcon?: ReactNode;
  fullWidth?: boolean;
}

export interface ButtonProps extends ButtonHTMLAttributes<HTMLButtonElement>, CommonProps {
  loading?: boolean;
}

function classesFor({ variant = 'secondary', size = 'md', fullWidth }: CommonProps) {
  return cn(BASE, VARIANTS[variant], variant !== 'link' && SIZES[size], fullWidth && 'w-full');
}

export const Button = forwardRef<HTMLButtonElement, ButtonProps>(function Button(
  {
    variant = 'secondary',
    size = 'md',
    loading = false,
    leftIcon,
    rightIcon,
    fullWidth,
    className,
    disabled,
    children,
    type = 'button',
    ...rest
  },
  ref,
) {
  return (
    <button
      ref={ref}
      type={type}
      disabled={disabled || loading}
      className={cn(classesFor({ variant, size, fullWidth }), className)}
      {...rest}
    >
      {loading ? <Loader2 className='h-3.5 w-3.5 animate-spin' aria-hidden /> : leftIcon}
      {children}
      {!loading && rightIcon}
    </button>
  );
});

export interface LinkButtonProps extends LinkProps, CommonProps {}

/**
 * A router link wearing the button's clothes. Navigation stays an anchor, so
 * middle-click and "open in new tab" keep working.
 */
export const LinkButton = forwardRef<HTMLAnchorElement, LinkButtonProps>(function LinkButton(
  {
    variant = 'secondary',
    size = 'md',
    leftIcon,
    rightIcon,
    fullWidth,
    className,
    children,
    ...rest
  },
  ref,
) {
  return (
    <Link ref={ref} className={cn(classesFor({ variant, size, fullWidth }), className)} {...rest}>
      {leftIcon}
      {children}
      {rightIcon}
    </Link>
  );
});
