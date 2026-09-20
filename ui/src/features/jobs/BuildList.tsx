/**
 * A job's builds as a narrow column beside the selected build.
 *
 * Deliberately not a table: the only questions here are "which build" and "did
 * it pass", and a column of links answers both while leaving the width to the
 * log. Each entry is a real link, so a build can be opened in a new tab.
 */
import { NavLink } from 'react-router-dom';
import { Tape, StatusMark } from '@/components/ui';
import { duration, relativeTime } from '@/lib/format';
import { cn } from '@/lib/cn';
import type { Build } from '@/types/jobs';

export interface BuildListProps {
  builds: Build[];
  /** Build id currently open, so the column can mark it. */
  selectedId?: string;
  /** Route for a build, e.g. ``(b) => `/jobs/${slug}/builds/${b.id}` ``. */
  linkTo: (build: Build) => string;
  className?: string;
}

export function BuildList({ builds, selectedId, linkTo, className }: BuildListProps) {
  return (
    <div className={cn('flex min-h-0 flex-col border-r border-border', className)}>
      <div className='flex shrink-0 items-center gap-2 border-b border-border px-3.5 py-2.5'>
        <span className='caption'>Builds</span>
        <Tape states={builds.map((b) => b.status).reverse()} className='ml-auto' />
      </div>
      <div className='scrollbar-thin min-h-0 flex-1 overflow-auto'>
        {builds.length === 0 && (
          <p className='px-3.5 py-4 text-xs leading-relaxed text-fg-subtle'>
            No builds yet. Run one from the Run tab.
          </p>
        )}
        {builds.map((build) => (
          <NavLink
            key={build.id}
            to={linkTo(build)}
            className={cn(
              'flex items-start gap-2.5 border-b border-border-subtle px-3.5 py-2.5 transition-colors',
              build.id === selectedId
                ? 'bg-surface-active shadow-[inset_2px_0_0_rgb(var(--color-fg))]'
                : 'hover:bg-surface-hover',
            )}
          >
            <StatusMark state={build.status} className='mt-[5px]' />
            <span className='min-w-0 flex-1'>
              <span className='flex items-baseline gap-2'>
                <span className='font-mono text-[12.5px] font-medium'>#{build.id}</span>
                <span className='ml-auto font-mono text-[11px] text-fg-subtle tabular-nums'>
                  {duration(build.durationMs)}
                </span>
              </span>
              <span className='mt-0.5 block truncate text-[11px] text-fg-subtle'>
                {relativeTime(build.startedAt)}
                {build.environment ? ` · ${build.environment}` : ''}
              </span>
            </span>
          </NavLink>
        ))}
      </div>
    </div>
  );
}
