/**
 * Author-side config editors: the type-specific half of the schema builder.
 *
 * The common half (name / label / env / required / description / default) is
 * rendered generically by `ParamEditor`, so a type only declares what makes it
 * different — options, ranges, patterns.
 */
import { GripVertical, Plus, X } from 'lucide-react';
import { cn } from '@/lib/cn';
import { TONE_TAG } from './tones';
import type { ParamConfigProps, ParamOption } from './types';

const CTRL =
  'w-full rounded border border-border bg-bg px-2 py-1.5 font-mono text-xs text-fg placeholder:text-fg-subtle focus-visible:border-primary focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-primary/20';

export function ConfigRow({ label, children }: { label: string; children: React.ReactNode }) {
  return (
    <label className='flex items-center gap-3'>
      <span className='w-24 shrink-0 font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
        {label}
      </span>
      <span className='flex-1'>{children}</span>
    </label>
  );
}

export function ConfigGrid({ children }: { children: React.ReactNode }) {
  return <div className='space-y-2'>{children}</div>;
}

function NumField({
  label,
  value,
  placeholder,
  onChange,
}: {
  label: string;
  value?: number;
  placeholder?: string;
  onChange: (v: number | undefined) => void;
}) {
  return (
    <label className='flex-1'>
      <span className='mb-1 block font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
        {label}
      </span>
      <input
        type='number'
        value={value ?? ''}
        placeholder={placeholder}
        onChange={(e) => onChange(e.target.value === '' ? undefined : Number(e.target.value))}
        className={CTRL}
      />
    </label>
  );
}

// --- options editor (shared by select / multiselect) ------------------------

const TONES: ParamOption['tone'][] = ['neutral', 'success', 'warning', 'danger'];

function OptionsEditor({ spec, onChange }: ParamConfigProps) {
  const options = spec.options ?? [];

  function patch(i: number, next: Partial<ParamOption>) {
    onChange({ options: options.map((o, j) => (i === j ? { ...o, ...next } : o)) });
  }
  function remove(i: number) {
    onChange({ options: options.filter((_, j) => j !== i) });
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
              className='cursor-grab text-fg-subtle hover:text-fg'
            >
              <GripVertical className='h-3.5 w-3.5' />
            </button>
            <input
              value={opt.value}
              onChange={(e) => patch(i, { value: e.target.value })}
              placeholder='value'
              className={cn(CTRL, 'w-24')}
            />
            <input
              value={opt.label ?? ''}
              onChange={(e) => patch(i, { label: e.target.value })}
              placeholder='label'
              className={cn(CTRL, 'flex-1')}
            />
            <input
              value={opt.hint ?? ''}
              onChange={(e) => patch(i, { hint: e.target.value })}
              placeholder='hint'
              className={cn(CTRL, 'flex-1 hidden lg:block')}
            />
            <button
              type='button'
              onClick={() => {
                const at = TONES.indexOf(opt.tone ?? 'neutral');
                patch(i, { tone: TONES[(at + 1) % TONES.length] });
              }}
              title='Cycle tone'
              className={cn(
                'rounded px-1.5 py-1 font-mono text-xxs',
                TONE_TAG[opt.tone ?? 'neutral']
              )}
            >
              {(opt.tone ?? 'neutral').slice(0, 4)}
            </button>
            <button
              type='button'
              onClick={() => remove(i)}
              className='text-fg-subtle hover:text-danger'
              aria-label='Remove option'
            >
              <X className='h-3.5 w-3.5' />
            </button>
          </div>
        ))}
        {options.length === 0 && (
          <p className='rounded border border-dashed border-border px-2 py-3 text-center text-xs text-fg-subtle'>
            No options yet
          </p>
        )}
      </div>
    </div>
  );
}

// --- per-type configs ------------------------------------------------------

export function StringConfig({ spec, onChange }: ParamConfigProps) {
  return (
    <ConfigGrid>
      <ConfigRow label='pattern'>
        <input
          value={spec.pattern ?? ''}
          onChange={(e) => onChange({ pattern: e.target.value || undefined })}
          placeholder='^[a-z0-9-]+$'
          className={CTRL}
        />
      </ConfigRow>
      <div className='flex gap-2'>
        <NumField
          label='min length'
          value={spec.minLength}
          onChange={(v) => onChange({ minLength: v })}
        />
        <NumField
          label='max length'
          value={spec.maxLength}
          onChange={(v) => onChange({ maxLength: v })}
        />
      </div>
      <label className='flex items-center gap-2 pt-1'>
        <input
          type='checkbox'
          checked={Boolean(spec.multiline)}
          onChange={(e) => onChange({ multiline: e.target.checked || undefined })}
          className='h-3.5 w-3.5 accent-primary'
        />
        <span className='text-xs text-fg-muted'>Multi-line (textarea)</span>
      </label>
    </ConfigGrid>
  );
}

export function NumberConfig({ spec, onChange }: ParamConfigProps) {
  return (
    <div className='flex gap-2'>
      <NumField label='min' value={spec.min} onChange={(v) => onChange({ min: v })} />
      <NumField label='max' value={spec.max} onChange={(v) => onChange({ max: v })} />
      <NumField label='step' value={spec.step} onChange={(v) => onChange({ step: v })} />
      <label className='flex-1'>
        <span className='mb-1 block font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
          unit
        </span>
        <input
          value={spec.unit ?? ''}
          onChange={(e) => onChange({ unit: e.target.value || undefined })}
          placeholder='ms'
          className={CTRL}
        />
      </label>
    </div>
  );
}

export function SelectConfig(props: ParamConfigProps) {
  return <OptionsEditor {...props} />;
}

export function MultiSelectConfig(props: ParamConfigProps) {
  const { spec, onChange } = props;
  return (
    <div className='space-y-3'>
      <OptionsEditor {...props} />
      <div className='flex gap-2'>
        <NumField
          label='min selected'
          value={spec.minSelected}
          onChange={(v) => onChange({ minSelected: v })}
        />
        <NumField
          label='max selected'
          value={spec.maxSelected}
          onChange={(v) => onChange({ maxSelected: v })}
        />
      </div>
    </div>
  );
}

export function SecretConfig({ spec, onChange }: ParamConfigProps) {
  return (
    <ConfigRow label='secretRef'>
      <input
        value={spec.secretRef ?? ''}
        onChange={(e) => onChange({ secretRef: e.target.value || undefined })}
        placeholder='secrets/deploy'
        className={CTRL}
      />
    </ConfigRow>
  );
}

export function FileConfig({ spec, onChange }: ParamConfigProps) {
  return (
    <ConfigRow label='accept'>
      <input
        value={spec.accept ?? ''}
        onChange={(e) => onChange({ accept: e.target.value || undefined })}
        placeholder='.md,.txt'
        className={CTRL}
      />
    </ConfigRow>
  );
}

export { CTRL as CONFIG_CTRL };
