import { type ReactNode } from 'react';
import { cn } from '@/lib/cn';

export interface PageHeaderProps {
  title: ReactNode;
  description?: ReactNode;
  /** Primary actions for the page, right-aligned on the title row. */
  actions?: ReactNode;
  /** A trail, or a single label naming where the page sits. */
  breadcrumb?: ReactNode;
  /**
   * Standing facts about the page's subject — counts, a repo, a last-sync time.
   * Compose with <Fact>; they wrap on narrow screens.
   */
  facts?: ReactNode;
  className?: string;
}

/**
 * The header of a list page. Non-scrolling: it stays put while the table below
 * it scrolls, so the page's identity and actions never leave the screen.
 */
export function PageHeader({
  title,
  description,
  actions,
  breadcrumb,
  facts,
  className,
}: PageHeaderProps) {
  return (
    <header
      className={cn('shrink-0 border-b border-border px-7 pb-5 pt-5', className)}
    >
      {breadcrumb && (
        <div className='flex min-h-[18px] flex-wrap items-center gap-1 text-xs text-fg-subtle'>
          {breadcrumb}
        </div>
      )}
      <div className='flex flex-wrap items-center gap-3'>
        <h1 className='m-0 mt-1 flex flex-wrap items-center gap-3 text-[22px] font-semibold leading-tight tracking-[-0.015em]'>
          {title}
        </h1>
        {actions && (
          <div className='ml-auto flex flex-wrap gap-1.5'>{actions}</div>
        )}
      </div>
      {description && (
        <p className='mt-1.5 max-w-[75ch] text-fg-muted'>{description}</p>
      )}
      {facts && (
        <div className='mt-2.5 flex flex-wrap items-center gap-x-[22px] gap-y-1.5 text-[12.5px] text-fg-muted'>
          {facts}
        </div>
      )}
    </header>
  );
}

export interface FactProps {
  /** A 13px lucide icon, dimmed to sit behind the value. */
  icon?: ReactNode;
  children: ReactNode;
  title?: string;
  className?: string;
}

/** One standing fact: an icon and a value, never a label–value pair. */
export function Fact({ icon, children, title, className }: FactProps) {
  return (
    <span
      title={title}
      className={cn(
        'inline-flex items-center gap-1.5 whitespace-nowrap [&>svg]:text-fg-subtle',
        className
      )}
    >
      {icon}
      {children}
    </span>
  );
}
