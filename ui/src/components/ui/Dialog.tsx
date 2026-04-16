import { Fragment, type ReactNode } from "react";
import { Dialog as HDialog, Transition } from "@headlessui/react";
import { X } from "lucide-react";
import { cn } from "@/lib/cn";

export interface DialogProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  size?: "sm" | "md" | "lg";
  initialFocus?: React.MutableRefObject<HTMLElement | null>;
}

const SIZE_CLASSES: Record<NonNullable<DialogProps["size"]>, string> = {
  sm: "max-w-md",
  md: "max-w-lg",
  lg: "max-w-2xl",
};

export function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = "md",
  initialFocus,
}: DialogProps) {
  return (
    <Transition appear show={open} as={Fragment}>
      <HDialog
        as="div"
        className="relative z-50"
        onClose={onClose}
        initialFocus={initialFocus}
      >
        <Transition.Child
          as={Fragment}
          enter="ease-out duration-150"
          enterFrom="opacity-0"
          enterTo="opacity-100"
          leave="ease-in duration-100"
          leaveFrom="opacity-100"
          leaveTo="opacity-0"
        >
          <div
            className="fixed inset-0 bg-black/60 backdrop-blur-sm"
            aria-hidden
          />
        </Transition.Child>

        <div className="fixed inset-0 overflow-y-auto">
          <div className="flex min-h-full items-center justify-center p-4">
            <Transition.Child
              as={Fragment}
              enter="ease-out duration-150"
              enterFrom="opacity-0 translate-y-2 scale-95"
              enterTo="opacity-100 translate-y-0 scale-100"
              leave="ease-in duration-100"
              leaveFrom="opacity-100"
              leaveTo="opacity-0 scale-95"
            >
              <HDialog.Panel
                className={cn(
                  "w-full rounded-lg border border-border bg-surface shadow-popover",
                  SIZE_CLASSES[size]
                )}
              >
                {(title || description) && (
                  <div className="flex items-start justify-between gap-4 border-b border-border px-5 py-4">
                    <div className="min-w-0 flex-1">
                      {title && (
                        <HDialog.Title className="text-base font-semibold text-fg">
                          {title}
                        </HDialog.Title>
                      )}
                      {description && (
                        <HDialog.Description className="mt-1 text-sm text-fg-muted">
                          {description}
                        </HDialog.Description>
                      )}
                    </div>
                    <button
                      type="button"
                      onClick={onClose}
                      className="rounded-md p-1 text-fg-muted hover:bg-surface-hover hover:text-fg"
                      aria-label="Close"
                    >
                      <X className="h-4 w-4" />
                    </button>
                  </div>
                )}
                <div className="px-5 py-4">{children}</div>
                {footer && (
                  <div className="flex items-center justify-end gap-2 border-t border-border px-5 py-3">
                    {footer}
                  </div>
                )}
              </HDialog.Panel>
            </Transition.Child>
          </div>
        </div>
      </HDialog>
    </Transition>
  );
}
