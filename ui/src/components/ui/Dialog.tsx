import { type ReactNode } from 'react';
import {
  Description,
  Dialog as HDialog,
  DialogBackdrop,
  DialogPanel,
  DialogTitle,
} from '@headlessui/react';
import { X } from 'lucide-react';
import { cn } from '@/lib/cn';

export interface DialogProps {
  open: boolean;
  onClose: () => void;
  title?: ReactNode;
  description?: ReactNode;
  children: ReactNode;
  footer?: ReactNode;
  size?: 'sm' | 'md' | 'lg';
}

const SIZE_CLASSES: Record<NonNullable<DialogProps['size']>, string> = {
  sm: 'max-w-md',
  md: 'max-w-lg',
  lg: 'max-w-2xl',
};

export function Dialog({
  open,
  onClose,
  title,
  description,
  children,
  footer,
  size = 'md',
}: DialogProps) {
  return (
    <HDialog open={open} onClose={onClose} className='relative z-50'>
      <DialogBackdrop
        transition
        className='fixed inset-0 bg-black/60 backdrop-blur-sm transition duration-150 ease-out data-[closed]:opacity-0 data-[leave]:duration-100 data-[leave]:ease-in'
      />
      <div className='fixed inset-0 overflow-y-auto'>
        <div className='flex min-h-full items-center justify-center p-4'>
          <DialogPanel
            transition
            className={cn(
              'w-full rounded-lg border border-border bg-surface shadow-popover transition duration-150 ease-out data-[closed]:translate-y-2 data-[closed]:scale-95 data-[closed]:opacity-0 data-[leave]:duration-100 data-[leave]:ease-in',
              SIZE_CLASSES[size],
            )}
          >
            {(title || description) && (
              <div className='flex items-start justify-between gap-4 border-b border-border px-5 py-4'>
                <div className='min-w-0 flex-1'>
                  {title && (
                    <DialogTitle className='text-base font-semibold text-fg'>{title}</DialogTitle>
                  )}
                  {description && (
                    <Description className='mt-1 text-sm text-fg-muted'>{description}</Description>
                  )}
                </div>
                <button
                  type='button'
                  onClick={onClose}
                  className='rounded-md p-1 text-fg-muted hover:bg-surface-hover hover:text-fg'
                  aria-label='Close'
                >
                  <X className='h-4 w-4' />
                </button>
              </div>
            )}
            <div className='px-5 py-4'>{children}</div>
            {footer && (
              <div className='flex items-center justify-end gap-2 border-t border-border px-5 py-3'>
                {footer}
              </div>
            )}
          </DialogPanel>
        </div>
      </div>
    </HDialog>
  );
}
