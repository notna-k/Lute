import {
  forwardRef,
  type HTMLAttributes,
  type TableHTMLAttributes,
  type TdHTMLAttributes,
  type ThHTMLAttributes,
} from "react";
import { cn } from "@/lib/cn";

export const Table = forwardRef<
  HTMLTableElement,
  TableHTMLAttributes<HTMLTableElement>
>(function Table({ className, ...props }, ref) {
  return (
    <div className="relative w-full overflow-auto scrollbar-thin">
      <table
        ref={ref}
        className={cn("w-full caption-bottom text-sm", className)}
        {...props}
      />
    </div>
  );
});

export const THead = forwardRef<
  HTMLTableSectionElement,
  HTMLAttributes<HTMLTableSectionElement>
>(function THead({ className, ...props }, ref) {
  return (
    <thead
      ref={ref}
      className={cn("bg-bg-subtle [&_th]:border-b [&_th]:border-border", className)}
      {...props}
    />
  );
});

export const TBody = forwardRef<
  HTMLTableSectionElement,
  HTMLAttributes<HTMLTableSectionElement>
>(function TBody({ className, ...props }, ref) {
  return (
    <tbody
      ref={ref}
      className={cn("[&_tr:last-child]:border-0", className)}
      {...props}
    />
  );
});

export const Tr = forwardRef<
  HTMLTableRowElement,
  HTMLAttributes<HTMLTableRowElement>
>(function Tr({ className, ...props }, ref) {
  return (
    <tr
      ref={ref}
      className={cn(
        "border-b border-border transition-colors hover:bg-surface-hover",
        className
      )}
      {...props}
    />
  );
});

export const Th = forwardRef<
  HTMLTableCellElement,
  ThHTMLAttributes<HTMLTableCellElement>
>(function Th({ className, ...props }, ref) {
  return (
    <th
      ref={ref}
      className={cn(
        "h-10 px-4 text-left align-middle text-xs font-semibold uppercase tracking-wide text-fg-muted",
        className
      )}
      {...props}
    />
  );
});

export const Td = forwardRef<
  HTMLTableCellElement,
  TdHTMLAttributes<HTMLTableCellElement>
>(function Td({ className, ...props }, ref) {
  return (
    <td
      ref={ref}
      className={cn("px-4 py-3 align-middle", className)}
      {...props}
    />
  );
});
