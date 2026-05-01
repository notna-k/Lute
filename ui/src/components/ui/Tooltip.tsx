import { useState, type ReactElement, type ReactNode, cloneElement } from 'react';
import { cn } from '@/lib/cn';

export interface TooltipProps {
  content: ReactNode;
  children: ReactElement;
  side?: 'top' | 'bottom' | 'left' | 'right';
  delay?: number;
  className?: string;
}

export function Tooltip({
  content,
  children,
  side = 'top',
  className,
}: TooltipProps) {
  const [open, setOpen] = useState(false);

  const show = () => setOpen(true);
  const hide = () => setOpen(false);

  const child = cloneElement(children, {
    onMouseEnter: show,
    onMouseLeave: hide,
    onFocus: show,
    onBlur: hide,
  });

  const sideClasses: Record<NonNullable<TooltipProps['side']>, string> = {
    top: 'bottom-full left-1/2 -translate-x-1/2 -translate-y-1.5',
    bottom: 'top-full left-1/2 -translate-x-1/2 translate-y-1.5',
    left: 'right-full top-1/2 -translate-y-1/2 -translate-x-1.5',
    right: 'left-full top-1/2 -translate-y-1/2 translate-x-1.5',
  };

  return (
    <span className='relative inline-flex'>
      {child}
      {open && content != null && (
        <span
          role='tooltip'
          className={cn(
            'pointer-events-none absolute z-50 whitespace-nowrap rounded-md bg-bg-inverse px-2 py-1 text-xs text-fg-inverse shadow-popover',
            'animate-fade-in',
            sideClasses[side],
            className
          )}
        >
          {content}
        </span>
      )}
    </span>
  );
}
