/**
 * Author-side config editors: the type-specific half of the schema editor.
 *
 * The common half (name / label / env / required / description / default) is
 * rendered generically by `ParamEditor`, so a type only declares what makes it
 * different.
 */
import { GripVertical, Plus, X } from 'lucide-react';
import { cn } from '@/lib/cn';
import { TONE_TAG, TONES } from './tones';
import type { ParamConfigProps } from './types';
import type { ParameterOption } from '@/types/jobs';

export const CONFIG_CTRL =
  'w-full rounded border border-border bg-bg px-2 py-1.5 font-mono text-xs text-fg placeholder:text-fg-subtle focus-visible:border-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20';

export function ConfigRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className='flex items-center gap-3'>
      <span className='w-24 shrink-0 font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
        {label}
      </span>
      <span className='min-w-0 flex-1'>{children}</span>
    </label>
  );
}

/** Options editor, shared by select and multiselect. */
function OptionsEditor({ field, onChange }: ParamConfigProps) {
  const options = field.options ?? [];

  function patch(i: number, next: Partial<ParameterOption>) {
    onChange({ options: options.map((o, j) => (i === j ? { ...o, ...next } : o)) });
  }
  function move(i: number, delta: number) {
    const j = i + delta;
    if (j < 0 || j >= options.length) return;
    const next = [...options];
    [next[i], next[j]] = [next[j], next[i]];
    onChange({ options: next });
  }

  return (
    <div>
      <div className='mb-1.5 flex items-center justify-between'>
        <span className='font-mono text-xxs uppercase tracking-wide text-fg-subtle'>Options</span>
        <button
          type='button'
          onClick={() => onChange({ options: [...options, { value: '', label: '' }] })}
          className='inline-flex items-center gap-1 rounded border border-dashed border-border px-1.5 py-0.5 font-mono text-xxs text-fg-muted hover:border-border-strong hover:text-fg'
        >
          <Plus className='h-3 w-3' /> add
        </button>
      </div>
      <div className='space-y-1'>
        {options.map((opt, i) => (
          <div key={i} className='flex items-center gap-1'>
            <button
              type='button'
              onClick={() => move(i, -1)}
              onDoubleClick={() => move(i, 1)}
              title='Click to move up, double-click to move down'
              className='shrink-0 cursor-grab text-fg-subtle hover:text-fg'
            >
              <GripVertical className='h-3.5 w-3.5' />
            </button>
            <input
              value={opt.value}
              onChange={(e) => patch(i, { value: e.target.value })}
              placeholder='value'
              className={cn(CONFIG_CTRL, 'w-24 shrink-0')}
            />
            <input
              value={opt.label ?? ''}
              onChange={(e) => patch(i, { label: e.target.value })}
              placeholder='label'
              className={cn(CONFIG_CTRL, 'min-w-0 flex-1')}
            />
            <input
              value={opt.hint ?? ''}
              onChange={(e) => patch(i, { hint: e.target.value })}
              placeholder='hint'
              className={cn(CONFIG_CTRL, 'hidden min-w-0 flex-1 2xl:block')}
            />
            <button
              type='button'
              onClick={() => {
                const at = TONES.indexOf((opt.tone ?? 'neutral') as (typeof TONES)[number]);
                patch(i, { tone: TONES[(at + 1) % TONES.length] });
              }}
              title='Cycle tone'
              className={cn(
                'shrink-0 rounded px-1.5 py-1 font-mono text-xxs',
                TONE_TAG[opt.tone ?? 'neutral']
              )}
            >
              {(opt.tone ?? 'neutral').slice(0, 4)}
            </button>
            <button
              type='button'
              onClick={() => onChange({ options: options.filter((_, j) => j !== i) })}
              className='shrink-0 text-fg-subtle hover:text-danger'
              aria-label='Remove option'
            >
              <X className='h-3.5 w-3.5' />
            </button>
          </div>
        ))}
        {options.length === 0 && (
          <p className='rounded border border-dashed border-border px-2 py-3 text-center text-xs text-fg-subtle'>
            No options yet — a select with no options can never be satisfied.
          </p>
        )}
      </div>
    </div>
  );
}

export function SelectConfig(props: ParamConfigProps) {
  return <OptionsEditor {...props} />;
}

export function SecretConfig({ field, onChange }: ParamConfigProps) {
  return (
    <ConfigRow label='secretRef'>
      <input
        value={field.secretRef ?? ''}
        onChange={(e) => onChange({ secretRef: e.target.value || undefined })}
        placeholder='secrets/deploy'
        className={CONFIG_CTRL}
      />
    </ConfigRow>
  );
}
