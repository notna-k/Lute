import { type HTMLAttributes, type ReactNode } from "react";
import { AlertCircle, CheckCircle2, Info, XCircle } from "lucide-react";
import { cn } from "@/lib/cn";

export type AlertTone = "info" | "success" | "warning" | "danger";

const TONE_STYLES: Record<AlertTone, string> = {
  info: "bg-info-subtle border-info/30 text-info-fg",
  success: "bg-success-subtle border-success/30 text-success-fg",
  warning: "bg-warning-subtle border-warning/30 text-warning-fg",
  danger: "bg-danger-subtle border-danger/30 text-danger-fg",
};

const TONE_ICONS: Record<AlertTone, typeof Info> = {
  info: Info,
  success: CheckCircle2,
  warning: AlertCircle,
  danger: XCircle,
};

export interface AlertProps
  extends Omit<HTMLAttributes<HTMLDivElement>, "title"> {
  tone?: AlertTone;
  title?: ReactNode;
  action?: ReactNode;
  icon?: ReactNode;
}

export function Alert({
  tone = "info",
  title,
  action,
  icon,
  className,
  children,
  ...rest
}: AlertProps) {
  const Icon = TONE_ICONS[tone];
  return (
    <div
      role={tone === "danger" ? "alert" : "status"}
      className={cn(
        "flex items-start gap-3 rounded-md border px-4 py-3 text-sm",
        TONE_STYLES[tone],
        className
      )}
      {...rest}
    >
      <span className="mt-0.5 shrink-0">
        {icon ?? <Icon className="h-4 w-4" />}
      </span>
      <div className="min-w-0 flex-1">
        {title && <div className="font-semibold">{title}</div>}
        {children && <div className={cn(title && "mt-0.5")}>{children}</div>}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  );
}
