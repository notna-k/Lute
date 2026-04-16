import { type HTMLAttributes } from "react";
import { cn } from "@/lib/cn";

export type BadgeTone =
  | "neutral"
  | "primary"
  | "success"
  | "warning"
  | "danger"
  | "info";

export type BadgeSize = "sm" | "md";

export interface BadgeProps extends HTMLAttributes<HTMLSpanElement> {
  tone?: BadgeTone;
  size?: BadgeSize;
  dot?: boolean;
}

const TONE_STYLES: Record<BadgeTone, string> = {
  neutral: "bg-bg-muted text-fg-muted border-border",
  primary: "bg-primary-subtle text-info-fg border-primary/20",
  success: "bg-success-subtle text-success-fg border-success/20",
  warning: "bg-warning-subtle text-warning-fg border-warning/20",
  danger: "bg-danger-subtle text-danger-fg border-danger/20",
  info: "bg-info-subtle text-info-fg border-info/20",
};

const TONE_DOT: Record<BadgeTone, string> = {
  neutral: "bg-fg-subtle",
  primary: "bg-primary",
  success: "bg-success",
  warning: "bg-warning",
  danger: "bg-danger",
  info: "bg-info",
};

const SIZES: Record<BadgeSize, string> = {
  sm: "text-xxs px-1.5 py-0.5",
  md: "text-xs px-2 py-0.5",
};

export function Badge({
  tone = "neutral",
  size = "md",
  dot,
  className,
  children,
  ...rest
}: BadgeProps) {
  return (
    <span
      className={cn(
        "inline-flex items-center gap-1.5 rounded-full border font-medium capitalize",
        TONE_STYLES[tone],
        SIZES[size],
        className
      )}
      {...rest}
    >
      {dot && (
        <span
          aria-hidden
          className={cn("h-1.5 w-1.5 rounded-full", TONE_DOT[tone])}
        />
      )}
      {children}
    </span>
  );
}
