/**
 * Parameter-schema core (PoC).
 *
 * PRODUCT.md §5 calls the parameter schema the crown jewel: one definition
 * drives both the trigger UI and server-side validation. The shape below is
 * built so that *adding an input type is a single file plus one registry
 * line* — no `switch` in a form component, no new interface fields, no
 * consumer to update. That is the whole answer to "Jenkins with 100500
 * plugins, but lightweight".
 */
import type { ComponentType } from 'react';
import type { LucideIcon } from 'lucide-react';

export type ParamTypeId =
  | 'string'
  | 'number'
  | 'bool'
  | 'select'
  | 'multiselect'
  | 'date'
  | 'datetime'
  | 'secret'
  | 'file'
  | 'slider';

export type ParamValue = string | number | boolean | string[];

export interface ParamOption {
  value: string;
  label?: string;
  /** Secondary line shown under the label in rich selects. */
  hint?: string;
  tone?: 'neutral' | 'success' | 'warning' | 'danger';
}

/**
 * One parameter in a job's input schema.
 *
 * Deliberately a flat bag rather than a discriminated union per type: the
 * registry decides which keys a given type reads, so a new type never forces
 * a change here or in anything that walks a schema generically (the YAML
 * writer, the validator, reordering, the env-var preview).
 */
export interface ParamSpec {
  /** Stable identity that survives renames — React keys and drag state use it. */
  id: string;
  type: ParamTypeId;
  /** Key in the trigger payload. */
  name: string;
  label: string;
  /** Environment variable the value is passed to the container as. */
  env: string;
  description?: string;
  required?: boolean;
  default?: ParamValue;
  options?: ParamOption[];
  // Constraint bag. Each type declares which of these it honours via `yamlKeys`.
  min?: number;
  max?: number;
  step?: number;
  unit?: string;
  minLength?: number;
  maxLength?: number;
  pattern?: string;
  multiline?: boolean;
  minSelected?: number;
  maxSelected?: number;
  secretRef?: string;
  accept?: string;
}

export type ParamValues = Record<string, ParamValue>;

export interface ParamInputProps {
  spec: ParamSpec;
  value: ParamValue;
  onChange: (value: ParamValue) => void;
  invalid?: boolean;
}

export interface ParamConfigProps {
  spec: ParamSpec;
  onChange: (patch: Partial<ParamSpec>) => void;
}

/**
 * Everything the app knows about one input type, in one place: how it is
 * authored, rendered, validated, serialised, and handed to the container.
 */
export interface ParamTypeDef {
  id: ParamTypeId;
  label: string;
  /** One-line description shown in the "add an input" palette. */
  blurb: string;
  icon: LucideIcon;
  /** Keys beyond the common set that this type writes to YAML, in order. */
  yamlKeys: (keyof ParamSpec)[];
  /** Partial spec seeded when an author adds this type to a schema. */
  seed: () => Partial<ParamSpec>;
  /** Value used when the spec declares no default. */
  emptyValue: (spec: ParamSpec) => ParamValue;
  /** The control a person triggering a build interacts with. */
  Input: ComponentType<ParamInputProps>;
  /** Type-specific controls in the schema editor (beyond label/name/env/required). */
  Config?: ComponentType<ParamConfigProps>;
  /** Returns an error message, or null when the value satisfies the spec. */
  validate?: (spec: ParamSpec, value: ParamValue) => string | null;
  /** How the value reaches the container. */
  toEnv: (spec: ParamSpec, value: ParamValue) => string;
}

// ---------------------------------------------------------------------------

export interface JobSource {
  repo: string;
  path: string;
  commit: string;
}

export interface JobDef {
  slug: string;
  name: string;
  description: string;
  queue: string;
  runtime: string;
  command: string;
  source: JobSource;
  params: ParamSpec[];
}

export type BuildStatus = 'running' | 'passed' | 'failed' | 'queued';

export interface Build {
  id: number;
  status: BuildStatus;
  startedAt: number;
  durationMs?: number;
  by: string;
  summary: string;
}
