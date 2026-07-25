/** Shared chrome for the PoCs: a switcher so all three are one click apart. */
import { Link, useLocation } from 'react-router-dom';
import { cn } from '@/lib/cn';
import { POCS } from '../pocList';

export function PocSwitcher() {
  const { pathname } = useLocation();
  return (
    <div className='mb-5 flex flex-wrap items-center gap-2 rounded-xl border border-dashed border-border bg-bg-subtle px-3 py-2'>
      <span className='font-mono text-xxs uppercase tracking-wider text-fg-subtle'>
        build-ux poc
      </span>
      {POCS.map((p) => {
        const active = pathname === p.path;
        return (
          <Link
            key={p.path}
            to={p.path}
            className={cn(
              'inline-flex items-center gap-2 rounded-md border px-2.5 py-1 text-xs transition-colors',
              active
                ? 'border-primary bg-primary-subtle text-primary'
                : 'border-border text-fg-muted hover:border-border-strong hover:text-fg'
            )}
          >
            <span className='font-mono font-bold'>{p.letter}</span>
            {p.name}
            <span className='hidden text-fg-subtle sm:inline'>· {p.tag}</span>
          </Link>
        );
      })}
    </div>
  );
}
