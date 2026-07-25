/**
 * The authoring form for one parameter.
 *
 * Two ideas worth keeping:
 *
 *  1. The *default* is set with the same control the runner will use. Authors
 *     never type `2026-07-25` into a text box to configure a date picker — they
 *     use the date picker. It doubles as a live preview of the field.
 *  2. `name` and `envVar` derive from the label until the author overrides
 *     them, so the common case is one field of typing, not three.
 */
import { useState } from 'react';
import { AlertTriangle, Link2, Link2Off } from 'lucide-react';
import { cn } from '@/lib/cn';
import { typeDef, TYPE_ORDER } from './registry';
import { CONFIG_CTRL, ConfigRow } from './configs';
import { envFromName, nameFromLabel } from './yaml';
import type { ParameterField, ParameterType, ParameterValue } from '@/types/jobs';

export interface ParamEditorProps {
  field: ParameterField;
  onChange: (patch: Partial<ParameterField>) => void;
  /** Names of the other parameters, for collision detection. */
  siblings: string[];
}

export function ParamEditor({ field, onChange, siblings }: ParamEditorProps) {
  const def = typeDef(field.type);
  const Config = def.Config;
  // Once an author edits name or env by hand, stop overwriting their choice.
  const [linked, setLinked] = useState(
    () => field.name === nameFromLabel(field.label) && field.envVar === envFromName(field.name)
  );
  const duplicate = siblings.includes(field.name);

  function setLabel(label: string) {
    if (!linked) return onChange({ label });
    const name = nameFromLabel(label);
    onChange({ label, name, envVar: envFromName(name) });
  }

  function switchType(type: ParameterType) {
    if (type === field.type) return;
    // Drop type-specific config that no longer applies — a stale `options`
    // array would silently resurface on a switch back.
    onChange({
      type,
      options: undefined,
      secretRef: undefined,
      default: undefined,
      ...typeDef(type).seed(),
    });
  }

  return (
    <div className='space-y-4'>
      <div className='flex flex-wrap gap-1'>
        {TYPE_ORDER.map((id) => {
          const t = typeDef(id);
          const Icon = t.icon;
          const active = id === field.type;
          return (
            <button
              key={id}
              type='button'
              onClick={() => switchType(id)}
              title={t.blurb}
              className={cn(
                'inline-flex items-center gap-1.5 rounded-md border px-2 py-1 text-xs transition-colors',
                active
                  ? 'border-primary bg-primary-subtle text-primary'
                  : 'border-border text-fg-muted hover:border-border-strong hover:text-fg'
              )}
            >
              <Icon className='h-3.5 w-3.5' />
              {t.label}
            </button>
          );
        })}
      </div>

      <div className='space-y-2'>
        <ConfigRow label='label'>
          <input
            value={field.label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder='Target environment'
            className={cn(CONFIG_CTRL, 'font-sans text-sm')}
          />
        </ConfigRow>

        <ConfigRow label='name / env'>
          <div className='flex items-center gap-1.5'>
            <input
              value={field.name}
              onChange={(e) => {
                setLinked(false);
                onChange({ name: e.target.value, envVar: envFromName(e.target.value) });
              }}
              className={cn(CONFIG_CTRL, duplicate && 'border-danger')}
            />
            <button
              type='button'
              onClick={() => setLinked((l) => !l)}
              title={linked ? 'Derived from the label — click to unlink' : 'Not linked to the label'}
              className={cn('shrink-0 p-1', linked ? 'text-primary' : 'text-fg-subtle')}
            >
              {linked ? <Link2 className='h-3.5 w-3.5' /> : <Link2Off className='h-3.5 w-3.5' />}
            </button>
            <input
              value={field.envVar}
              onChange={(e) => {
                setLinked(false);
                onChange({ envVar: e.target.value });
              }}
              className={cn(CONFIG_CTRL, 'text-primary')}
            />
          </div>
        </ConfigRow>

        {duplicate && (
          <p className='flex items-center gap-1.5 pl-[6.75rem] text-xs text-danger'>
            <AlertTriangle className='h-3.5 w-3.5' /> Another parameter already uses this name
          </p>
        )}

        <ConfigRow label='help text'>
          <input
            value={field.description ?? ''}
            onChange={(e) => onChange({ description: e.target.value || undefined })}
            placeholder='Shown under the input'
            className={cn(CONFIG_CTRL, 'font-sans text-sm')}
          />
        </ConfigRow>

        <ConfigRow label='required'>
          <button
            type='button'
            role='switch'
            aria-checked={Boolean(field.required)}
            onClick={() => onChange({ required: !field.required })}
            className='inline-flex items-center gap-2'
          >
            <span
              className={cn(
                'relative h-5 w-9 rounded-full border transition-colors',
                field.required ? 'border-warning bg-warning-subtle' : 'border-border bg-surface-hover'
              )}
            >
              <span
                className={cn(
                  'absolute top-0.5 h-3.5 w-3.5 rounded-full transition-all',
                  field.required ? 'left-[1.125rem] bg-warning' : 'left-0.5 bg-fg-subtle'
                )}
              />
            </span>
            <span className='font-mono text-xxs text-fg-muted'>
              {field.required ? 'true' : 'false'}
            </span>
          </button>
        </ConfigRow>
      </div>

      {Config && (
        <div className='rounded-lg border border-border-subtle bg-bg-subtle p-3'>
          <Config field={field} onChange={onChange} />
        </div>
      )}

      {/* Default value, set with the real control. */}
      {field.type !== 'secret' && (
        <div className='rounded-lg border border-dashed border-border p-3'>
          <div className='mb-2 flex items-center gap-2'>
            <span className='font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
              default value
            </span>
            {field.default !== undefined && field.default !== '' && (
              <button
                type='button'
                onClick={() => onChange({ default: undefined })}
                className='ml-auto font-mono text-xxs text-fg-subtle hover:text-danger'
              >
                clear
              </button>
            )}
          </div>
          <def.Input
            field={field}
            value={(field.default as ParameterValue) ?? def.emptyValue(field)}
            onChange={(v) => onChange({ default: v })}
          />
        </div>
      )}
    </div>
  );
}
