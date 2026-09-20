import {
  forwardRef,
  type HTMLAttributes,
  type TableHTMLAttributes,
  type TdHTMLAttributes,
  type ThHTMLAttributes,
} from 'react';
import { useNavigate } from 'react-router-dom';
import { cn } from '@/lib/cn';

/**
 * Dense data table.
 *
 * Headers stick to the top of the scroll pane, rows are separated by the faint
 * hairline rather than a stripe, and cells do not wrap: these tables are read
 * by scanning a column, so a row that grows to two lines breaks the rhythm.
 * Edge cells carry the page gutter so the first column lines up with the
 * page heading above it.
 */
export const Table = forwardRef<HTMLTableElement, TableHTMLAttributes<HTMLTableElement>>(
  function Table({ className, ...props }, ref) {
    return (
      <table
        ref={ref}
        className={cn(
          'w-full border-collapse text-[13px]',
          '[&_td:first-child]:pl-7 [&_th:first-child]:pl-7',
          '[&_td:last-child]:pr-7 [&_th:last-child]:pr-7',
          className,
        )}
        {...props}
      />
    );
  },
);

export const THead = forwardRef<HTMLTableSectionElement, HTMLAttributes<HTMLTableSectionElement>>(
  function THead({ className, ...props }, ref) {
    return <thead ref={ref} className={className} {...props} />;
  },
);

export const TBody = forwardRef<HTMLTableSectionElement, HTMLAttributes<HTMLTableSectionElement>>(
  function TBody({ className, ...props }, ref) {
    return <tbody ref={ref} className={className} {...props} />;
  },
);

export const Tr = forwardRef<HTMLTableRowElement, HTMLAttributes<HTMLTableRowElement>>(function Tr(
  { className, ...props },
  ref,
) {
  return <tr ref={ref} className={className} {...props} />;
});

export const Th = forwardRef<HTMLTableCellElement, ThHTMLAttributes<HTMLTableCellElement>>(
  function Th({ className, ...props }, ref) {
    return (
      <th
        ref={ref}
        className={cn(
          'caption sticky top-0 z-[1] whitespace-nowrap border-b border-border bg-bg px-3.5 py-2.5 text-left',
          className,
        )}
        {...props}
      />
    );
  },
);

export const Td = forwardRef<HTMLTableCellElement, TdHTMLAttributes<HTMLTableCellElement>>(
  function Td({ className, ...props }, ref) {
    return (
      <td
        ref={ref}
        className={cn(
          // Row height follows the density chosen in Settings; see tokens.css.
          'whitespace-nowrap border-b border-border-subtle px-3.5 py-[var(--density-row-y)] align-middle',
          className,
        )}
        {...props}
      />
    );
  },
);

export interface RowLinkProps extends HTMLAttributes<HTMLTableRowElement> {
  /** Clicking anywhere in the row that is not itself interactive navigates here. */
  to: string;
}

/**
 * A whole-row link.
 *
 * The row is not an anchor (HTML forbids it inside a table), so it navigates on
 * click while still containing real links — the primary cell keeps its own
 * anchor, which is what keyboard users tab to and what "open in new tab" uses.
 */
export const RowLink = forwardRef<HTMLTableRowElement, RowLinkProps>(function RowLink(
  { to, className, onClick, ...props },
  ref,
) {
  const navigate = useNavigate();
  return (
    <tr
      ref={ref}
      className={cn('cursor-pointer [&:hover>td]:bg-surface-hover', className)}
      onClick={(event) => {
        onClick?.(event);
        if (event.defaultPrevented) return;
        // Let a nested control handle its own click.
        if ((event.target as HTMLElement).closest('a,button,input,label,select')) return;
        navigate(to);
      }}
      {...props}
    />
  );
});
