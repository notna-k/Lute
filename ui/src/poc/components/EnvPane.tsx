/**
 * "What will actually run" — the resolved env block for the values currently
 * in the form. Jenkins makes you read the console output to learn this; here
 * it is visible before you press the button.
 */
import { cn } from '@/lib/cn';
import { toEnvPairs } from '../params/registry';
import type { ParamSpec, ParamValue } from '../params/types';

export interface EnvPaneProps {
  specs: ParamSpec[];
  values: Record<string, ParamValue>;
  command: string;
  runtime: string;
  className?: string;
  /** Param name to highlight, when the layout tracks focus. */
  focusName?: string | null;
}

export function EnvPane({
  specs,
  values,
  command,
  runtime,
  className,
  focusName,
}: EnvPaneProps) {
  const pairs = toEnvPairs(specs, values);
  return (
    <div className={cn('rounded-xl border border-border bg-bg-subtle', className)}>
      <div className='border-b border-border px-3 py-2'>
        <span className='font-mono text-xxs uppercase tracking-wider text-fg-muted'>
          resolved environment
        </span>
      </div>
      <div className='overflow-x-auto px-3 py-2.5 font-mono text-xs leading-relaxed'>
        <div className='text-fg-subtle'>docker run --rm \</div>
        {pairs.map((p, i) => (
          <div
            key={p.key}
            className={cn(
              '-mx-1 whitespace-nowrap px-1',
              specs[i]?.name === focusName && 'rounded bg-primary-subtle'
            )}
          >
            <span className='text-fg-subtle'> -e </span>
            <span className='text-info'>{p.key}</span>
            <span className='text-fg-subtle'>=</span>
            {p.masked ? (
              <span className='text-warning'>$(secret)</span>
            ) : p.value === '' ? (
              <span className='text-fg-subtle italic'>''</span>
            ) : (
              <span className='text-success'>{JSON.stringify(p.value)}</span>
            )}
            <span className='text-fg-subtle'> \</span>
          </div>
        ))}
        <div className='text-fg'>
          {'  '}
          <span className='text-fg-subtle'>{runtime}</span> {command}
        </div>
      </div>
    </div>
  );
}
