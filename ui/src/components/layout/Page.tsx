import { forwardRef, type HTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

// A page is non-scrolling frame parts plus exactly one PageScroll, which keeps sticky headers working.

/** The scrolling region of a page. One per page; it fills the leftover height. */
export const PageScroll = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function PageScroll({ className, ...props }, ref) {
    return (
      <div
        ref={ref}
        className={cn(
          'scrollbar-thin min-h-0 flex-1 overflow-auto max-md:overflow-visible',
          className,
        )}
        {...props}
      />
    );
  },
);

/** Standard content padding inside a PageScroll, matching the header's gutter. */
export const PageBody = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function PageBody({ className, ...props }, ref) {
    return (
      <div
        ref={ref}
        className={cn('px-7 pb-14 pt-[var(--density-body-top)] max-md:px-4', className)}
        {...props}
      />
    );
  },
);

export interface SectionProps extends HTMLAttributes<HTMLElement> {
  title?: string;
  /** Right-aligned note or control on the section's heading row. */
  aside?: React.ReactNode;
}

/** A titled block inside a PageBody. */
export function Section({ title, aside, className, children, ...rest }: SectionProps) {
  return (
    <section className={cn('mt-[var(--density-section-gap)] first:mt-0', className)} {...rest}>
      {(title || aside) && (
        <h3 className='mb-2 flex items-center gap-2.5'>
          {title && <span className='caption'>{title}</span>}
          {aside && <span className='ml-auto text-xs font-normal text-fg-muted'>{aside}</span>}
        </h3>
      )}
      {children}
    </section>
  );
}
