import { forwardRef, type HTMLAttributes } from 'react';
import { cn } from '@/lib/cn';

/**
 * The strip under a page header that holds a page's filters. It carries its own
 * bottom hairline and never scrolls, so it reads as part of the page frame
 * rather than as the first row of content.
 *
 * List pages do not use it directly — they compose FilterBar, which fills it
 * with the shared search / scope / facet vocabulary.
 */
export const Toolbar = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function Toolbar({ className, ...props }, ref) {
    return (
      <div
        ref={ref}
        className={cn(
          'flex shrink-0 flex-wrap items-center gap-2.5 border-b border-border px-7 py-[var(--density-toolbar-y)] max-md:px-4',
          className
        )}
        {...props}
      />
    );
  }
);
