import { forwardRef, type InputHTMLAttributes } from "react";
import { Check } from "lucide-react";
import { cn } from "@/lib/cn";

export interface CheckboxProps
  extends Omit<InputHTMLAttributes<HTMLInputElement>, "type"> {
  label?: string;
}

export const Checkbox = forwardRef<HTMLInputElement, CheckboxProps>(
  function Checkbox({ className, label, id, checked, ...props }, ref) {
    return (
      <label
        htmlFor={id}
        className={cn(
          "inline-flex cursor-pointer select-none items-center gap-2",
          props.disabled && "cursor-not-allowed opacity-50",
          className
        )}
      >
        <span className="relative inline-flex h-4 w-4 items-center justify-center">
          <input
            ref={ref}
            id={id}
            type="checkbox"
            checked={checked}
            className="peer sr-only"
            {...props}
          />
          <span
            aria-hidden
            className={cn(
              "h-4 w-4 rounded border border-border bg-surface transition-colors",
              "peer-checked:border-primary peer-checked:bg-primary",
              "peer-focus-visible:ring-2 peer-focus-visible:ring-primary/30"
            )}
          />
          {checked && (
            <Check
              className="pointer-events-none absolute h-3 w-3 text-fg-onPrimary"
              aria-hidden
            />
          )}
        </span>
        {label && <span className="text-sm text-fg">{label}</span>}
      </label>
    );
  }
);
