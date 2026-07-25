/**
 * The authoring form for one parameter.
 *
 * Two ideas worth keeping regardless of which layout wins:
 *
 *  1. The *default* is set with the same control the runner will use. Authors
 *     never type `2026-07-25` into a text box to configure a date picker — they
 *     use the date picker. It doubles as a live preview of the field.
 *  2. `name` and `env` derive from the label until the author overrides them,
 *     so the common case is one field of typing, not three.
 */
import { useState } from 'react';
import { AlertTriangle, Link2, Link2Off } from 'lucide-react';
import { cn } from '@/lib/cn';
import { typeDef, TYPE_ORDER } from '../params/registry';
import { CONFIG_CTRL, ConfigRow } from '../params/configs';
import { envFromName, nameFromLabel } from '../params/yaml';
import type { ParamSpec, ParamTypeId, ParamValue } from '../params/types';

export interface ParamEditorProps {
  spec: ParamSpec;
  onChange: (patch: Partial<ParamSpec>) => void;
  /** Names of the other params — used to flag collisions. */
  siblings: string[];
}

export function ParamEditor({ spec, onChange, siblings }: ParamEditorProps) {
  const def = typeDef(spec.type);
  const Config = def.Config;
  // Once an author edits name/env by hand we stop overwriting their choice.
  const [linked, setLinked] = useState(
    () => spec.name === nameFromLabel(spec.label) && spec.env === envFromName(spec.name)
  );
  const duplicate = siblings.filter((n) => n === spec.name).length > 0;

  function setLabel(label: string) {
    if (!linked) return onChange({ label });
    const name = nameFromLabel(label);
    onChange({ label, name, env: envFromName(name) });
  }

  function switchType(type: ParamTypeId) {
    if (type === spec.type) return;
    // Carry over identity, drop type-specific config that no longer applies —
    // keeping a stale `options` array would silently resurface on a switch back.
    const next = typeDef(type);
    onChange({
      type,
      options: undefined,
      min: undefined,
      max: undefined,
      step: undefined,
      unit: undefined,
      pattern: undefined,
      minLength: undefined,
      maxLength: undefined,
      multiline: undefined,
      minSelected: undefined,
      maxSelected: undefined,
      secretRef: undefined,
      accept: undefined,
      default: undefined,
      ...next.seed(),
    });
  }

  return (
    <div className='space-y-4'>
      {/* type picker */}
      <div className='flex flex-wrap gap-1'>
        {TYPE_ORDER.map((id) => {
          const t = typeDef(id);
          const Icon = t.icon;
          const active = id === spec.type;
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
            value={spec.label}
            onChange={(e) => setLabel(e.target.value)}
            placeholder='Target environment'
            className={cn(CONFIG_CTRL, 'font-sans text-sm')}
          />
        </ConfigRow>

        <ConfigRow label='name / env'>
          <div className='flex items-center gap-1.5'>
            <input
              value={spec.name}
              onChange={(e) => {
                setLinked(false);
                onChange({ name: e.target.value, env: envFromName(e.target.value) });
              }}
              className={cn(CONFIG_CTRL, duplicate && 'border-danger')}
            />
            <button
              type='button'
              onClick={() => setLinked((l) => !l)}
              title={linked ? 'Derived from label — click to unlink' : 'Linked off'}
              className={cn('shrink-0 p-1', linked ? 'text-primary' : 'text-fg-subtle')}
            >
              {linked ? <Link2 className='h-3.5 w-3.5' /> : <Link2Off className='h-3.5 w-3.5' />}
            </button>
            <input
              value={spec.env}
              onChange={(e) => {
                setLinked(false);
                onChange({ env: e.target.value });
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
            value={spec.description ?? ''}
            onChange={(e) => onChange({ description: e.target.value || undefined })}
            placeholder='Shown under the input'
            className={cn(CONFIG_CTRL, 'font-sans text-sm')}
          />
        </ConfigRow>

        <ConfigRow label='required'>
          <button
            type='button'
            role='switch'
            aria-checked={Boolean(spec.required)}
            onClick={() => onChange({ required: !spec.required || undefined })}
            className='inline-flex items-center gap-2'
          >
            <span
              className={cn(
                'relative h-5 w-9 rounded-full border transition-colors',
                spec.required ? 'border-warning bg-warning-subtle' : 'border-border bg-surface-hover'
              )}
            >
              <span
                className={cn(
                  'absolute top-0.5 h-3.5 w-3.5 rounded-full transition-all',
                  spec.required ? 'left-[1.125rem] bg-warning' : 'left-0.5 bg-fg-subtle'
                )}
              />
            </span>
            <span className='font-mono text-xxs text-fg-muted'>
              {spec.required ? 'true' : 'false'}
            </span>
          </button>
        </ConfigRow>
      </div>

      {Config && (
        <div className='rounded-lg border border-border-subtle bg-bg-subtle p-3'>
          <Config spec={spec} onChange={onChange} />
        </div>
      )}

      {/* Default value, set with the real control. */}
      {spec.type !== 'secret' && (
        <div className='rounded-lg border border-dashed border-border p-3'>
          <div className='mb-2 flex items-center gap-2'>
            <span className='font-mono text-xxs uppercase tracking-wide text-fg-subtle'>
              default value
            </span>
            {spec.default !== undefined && spec.default !== '' && (
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
            spec={spec}
            value={(spec.default as ParamValue) ?? def.emptyValue(spec)}
            onChange={(v) => onChange({ default: v })}
          />
        </div>
      )}
    </div>
  );
}
