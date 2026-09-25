import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from 'react';
import { Link } from 'react-router-dom';
import { cn } from '@/lib/cn';

// Transient confirmations with one follow-up link. Not an error channel: errors stay by their control.

interface ToastLink {
  to: string;
  label: string;
}

export interface ToastOptions {
  /** A marker or icon shown before the message. */
  icon?: ReactNode;
  message: ReactNode;
  link?: ToastLink;
  /** Milliseconds on screen. */
  duration?: number;
}

interface ToastEntry extends ToastOptions {
  id: number;
}

interface ToastContextValue {
  toast: (options: ToastOptions) => void;
}

const ToastContext = createContext<ToastContextValue | null>(null);

const MAX_VISIBLE = 4;
const DEFAULT_DURATION = 5200;

export function ToastProvider({ children }: { children: ReactNode }) {
  const [entries, setEntries] = useState<ToastEntry[]>([]);
  const nextId = useRef(1);
  const timers = useRef(new Map<number, ReturnType<typeof setTimeout>>());

  const dismiss = useCallback((id: number) => {
    setEntries((prev) => prev.filter((entry) => entry.id !== id));
    const timer = timers.current.get(id);
    if (timer) {
      clearTimeout(timer);
      timers.current.delete(id);
    }
  }, []);

  const toast = useCallback(
    (options: ToastOptions) => {
      const id = nextId.current++;
      setEntries((prev) => [...prev, { ...options, id }].slice(-MAX_VISIBLE));
      timers.current.set(
        id,
        setTimeout(() => dismiss(id), options.duration ?? DEFAULT_DURATION),
      );
    },
    [dismiss],
  );

  // Clear pending timers on unmount so a late fire cannot set state on a gone provider.
  useEffect(
    () => () => {
      timers.current.forEach(clearTimeout);
      timers.current.clear();
    },
    [],
  );

  const value = useMemo(() => ({ toast }), [toast]);

  return (
    <ToastContext.Provider value={value}>
      {children}
      <div
        aria-live='polite'
        className='pointer-events-none fixed bottom-4 right-4 z-[60] grid justify-items-end gap-2'
      >
        {entries.map((entry) => (
          <div
            key={entry.id}
            role='status'
            className='pointer-events-auto flex min-w-[260px] max-w-[420px] animate-slide-up items-center gap-2.5 border border-border bg-surface px-3 py-2.5 text-[12.5px] shadow-overlay'
          >
            {entry.icon}
            <span className='min-w-0'>{entry.message}</span>
            {entry.link && (
              <Link
                to={entry.link.to}
                onClick={() => dismiss(entry.id)}
                className='ml-auto whitespace-nowrap font-medium underline underline-offset-[3px]'
              >
                {entry.link.label}
              </Link>
            )}
          </div>
        ))}
      </div>
    </ToastContext.Provider>
  );
}

/** Returns `toast(...)`. Safe to call outside a provider — it no-ops. */
export function useToast(): ToastContextValue {
  return useContext(ToastContext) ?? NO_TOASTS;
}

const NO_TOASTS: ToastContextValue = { toast: () => undefined };

/** Shared class for the bold monospace subject inside a toast message. */
export const toastSubject = cn('font-mono font-medium');
