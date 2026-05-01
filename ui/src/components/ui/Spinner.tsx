import { Loader2 } from 'lucide-react';
import { cn } from '@/lib/cn';

export interface SpinnerProps {
  size?: number;
  className?: string;
  label?: string;
}

export function Spinner({ size = 20, className, label }: SpinnerProps) {
  return (
    <span
      role='status'
      aria-label={label ?? 'Loading'}
      className={cn('inline-flex items-center justify-center', className)}
    >
      <Loader2 className='animate-spin text-fg-muted' style={{ width: size, height: size }} />
    </span>
  );
}
