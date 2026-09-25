import { type ReactNode } from 'react';
import { ChevronRight } from 'lucide-react';
import { Link } from 'react-router-dom';
import { cn } from '@/lib/cn';

interface Crumb {
  label: string;
  to?: string;
}

export interface DetailHeaderProps {
  /** Ancestors of this record, rendered inline before the title. */
  crumbs?: Crumb[];
  /** The record's identity — monospaced, because it is an identifier. */
  title: ReactNode;
  /** A short description, truncated to one line: the header must not grow. */
  subtitle?: ReactNode;
  /** Status or drift markers sitting beside the title. */
  tags?: ReactNode;
  actions?: ReactNode;
  /** The record's tab strip. */
  tabs?: ReactNode;
  /** Standing facts, right-aligned on the tab row where there is spare width. */
  meta?: ReactNode;
  className?: string;
}

/**
 * The header of a single record's page — a job, a worker, a build.
 *
 * Two rows and no more. The first answers "what am I looking at and what can I
 * do to it"; the second carries the tabs, with the record's unchanging facts
 * parked in the space beside them rather than in a block of their own. That
 * keeps the frame short enough that a build log gets the screen.
 */
export function DetailHeader({
  crumbs,
  title,
  subtitle,
  tags,
  actions,
  tabs,
  meta,
  className,
}: DetailHeaderProps) {
  return (
    <header
      className={cn(
        'shrink-0 border-b border-border px-7 pt-[var(--density-toolbar-y)] max-md:px-4',
        className,
      )}
    >
      <div className='flex flex-wrap items-center gap-3'>
        <div className='flex min-w-0 flex-1 items-baseline gap-2.5'>
          {crumbs && crumbs.length > 0 && (
            <span className='flex shrink-0 items-center gap-1 whitespace-nowrap text-[12.5px] text-fg-subtle'>
              {crumbs.map((crumb) => (
                <span key={crumb.label} className='flex items-center gap-1'>
                  {crumb.to ? (
                    <Link to={crumb.to} className='hover:text-fg'>
                      {crumb.label}
                    </Link>
                  ) : (
                    crumb.label
                  )}
                  <ChevronRight className='h-3 w-3' />
                </span>
              ))}
            </span>
          )}
          <h1 className='m-0 whitespace-nowrap font-mono text-[19px] font-semibold leading-tight tracking-[-0.02em]'>
            {title}
          </h1>
          {tags}
          {subtitle && (
            <span className='min-w-0 truncate text-[12.5px] text-fg-subtle'>{subtitle}</span>
          )}
        </div>
        {actions && <div className='flex shrink-0 gap-1.5'>{actions}</div>}
      </div>

      <div className='mt-3 flex flex-wrap items-end gap-x-6 gap-y-1'>
        {tabs}
        {meta && (
          <div className='ml-auto flex flex-wrap items-center justify-end gap-x-4 gap-y-1 pb-2.5 text-xs text-fg-subtle [&_a:hover]:text-fg'>
            {meta}
          </div>
        )}
      </div>
    </header>
  );
}
