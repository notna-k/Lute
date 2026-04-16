import { forwardRef, type ButtonHTMLAttributes } from "react";
import { cn } from "@/lib/cn";

export type IconButtonVariant = "ghost" | "outline" | "solid" | "danger";
export type IconButtonSize = "sm" | "md";

export interface IconButtonProps
  extends ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: IconButtonVariant;
  size?: IconButtonSize;
  label: string;
}

const VARIANTS: Record<IconButtonVariant, string> = {
  ghost: "bg-transparent text-fg-muted hover:bg-surface-hover hover:text-fg",
  outline:
    "bg-transparent text-fg-muted border border-border hover:bg-surface-hover hover:text-fg",
  solid:
    "bg-surface text-fg border border-border hover:bg-surface-hover shadow-sm",
  danger:
    "bg-transparent text-danger hover:bg-danger/10",
};

const SIZES: Record<IconButtonSize, string> = {
  sm: "h-7 w-7",
  md: "h-8 w-8",
};

export const IconButton = forwardRef<HTMLButtonElement, IconButtonProps>(
  function IconButton(
    {
      variant = "ghost",
      size = "md",
      label,
      className,
      type = "button",
      children,
      ...rest
    },
    ref
  ) {
    return (
      <button
        ref={ref}
        type={type}
        aria-label={label}
        title={label}
        className={cn(
          "inline-flex items-center justify-center rounded-md transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary focus-visible:ring-offset-2 focus-visible:ring-offset-bg disabled:cursor-not-allowed disabled:opacity-50",
          VARIANTS[variant],
          SIZES[size],
          className
        )}
        {...rest}
      >
        {children}
      </button>
    );
  }
);
