import { forwardRef, type HTMLAttributes } from 'react';
import { Search } from 'lucide-react';
import { cn } from '@/lib/cn';
import { Input, type InputProps } from './Input';

/**
 * The filter strip under a page header: search, segmented filters, and a result
 * count pushed to the right. Fixed height and its own bottom hairline, so it
 * reads as part of the page frame rather than as the first row of content.
 */
export const Toolbar = forwardRef<HTMLDivElement, HTMLAttributes<HTMLDivElement>>(
  function Toolbar({ className, ...props }, ref) {
    return (
      <div
        ref={ref}
        className={cn(
          'flex shrink-0 flex-wrap items-center gap-2.5 border-b border-border px-7 py-3',
          className
        )}
        {...props}
      />
    );
  }
);

export interface SearchInputProps extends Omit<InputProps, 'leftIcon'> {
  className?: string;
}

/** A search box with the magnifier baked in, so every filter looks the same. */
export const SearchInput = forwardRef<HTMLInputElement, SearchInputProps>(
  function SearchInput({ className, ...props }, ref) {
    return (
      <Input
        ref={ref}
        type='search'
        autoComplete='off'
        leftIcon={<Search className='h-3.5 w-3.5' />}
        className={cn('w-[280px] max-w-full', className)}
        {...props}
      />
    );
  }
);
