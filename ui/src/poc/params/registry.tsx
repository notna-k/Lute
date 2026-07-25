/**
 * The input-type registry.
 *
 * Adding a type means adding one entry here (plus its control and, if it needs
 * one, its config editor). Nothing else in the app enumerates types: the form
 * renderer, the schema builder, the validator, the YAML writer and the env
 * preview all go through this table. That is the "no plugin sprawl" bet — the
 * extension point is a data structure, not a plugin host.
 */
import {
  Calendar,
  CalendarClock,
  FileUp,
  Hash,
  KeyRound,
  List,
  ListChecks,
  SlidersHorizontal,
  ToggleLeft,
  Type,
} from 'lucide-react';
import {
  DateInput,
  DateTimeInput,
  FileInput,
  MultiSelectInput,
  NumberInput,
  SecretInput,
  SelectInput,
  SliderInput,
  TextInput,
  ToggleInput,
} from './controls';
import {
  FileConfig,
  MultiSelectConfig,
  NumberConfig,
  SecretConfig,
  SelectConfig,
  StringConfig,
} from './configs';
import type { ParamSpec, ParamTypeDef, ParamTypeId, ParamValue } from './types';

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

function asNumber(value: ParamValue): number {
  return typeof value === 'number' ? value : Number(value);
}

export const PARAM_TYPES: Record<ParamTypeId, ParamTypeDef> = {
  string: {
    id: 'string',
    label: 'Text',
    blurb: 'Free text, optionally pattern-checked',
    icon: Type,
    yamlKeys: ['pattern', 'minLength', 'maxLength', 'multiline'],
    seed: () => ({ default: '' }),
    emptyValue: () => '',
    Input: TextInput,
    Config: StringConfig,
    validate: (spec, value) => {
      const s = String(value ?? '');
      if (spec.minLength && s.length < spec.minLength) {
        return `Must be at least ${spec.minLength} characters`;
      }
      if (spec.maxLength && s.length > spec.maxLength) {
        return `Must be at most ${spec.maxLength} characters`;
      }
      // An invalid author-supplied pattern must not take the form down.
      if (spec.pattern && s) {
        try {
          if (!new RegExp(spec.pattern).test(s)) return `Must match ${spec.pattern}`;
        } catch {
          return null;
        }
      }
      return null;
    },
    toEnv: (_s, v) => String(v ?? ''),
  },

  number: {
    id: 'number',
    label: 'Number',
    blurb: 'Numeric input with range and step',
    icon: Hash,
    yamlKeys: ['min', 'max', 'step', 'unit'],
    seed: () => ({ default: 0, step: 1 }),
    emptyValue: () => '',
    Input: NumberInput,
    Config: NumberConfig,
    validate: (spec, value) => {
      if (value === '') return null; // emptiness is the `required` check's business
      const n = asNumber(value);
      if (!Number.isFinite(n)) return 'Must be a number';
      if (spec.min !== undefined && n < spec.min) return `Must be ≥ ${spec.min}`;
      if (spec.max !== undefined && n > spec.max) return `Must be ≤ ${spec.max}`;
      return null;
    },
    toEnv: (_s, v) => String(v ?? ''),
  },

  slider: {
    id: 'slider',
    label: 'Slider',
    blurb: 'Bounded number, dragged rather than typed',
    icon: SlidersHorizontal,
    yamlKeys: ['min', 'max', 'step', 'unit'],
    seed: () => ({ min: 0, max: 100, step: 5, default: 50 }),
    emptyValue: (spec) => spec.min ?? 0,
    Input: SliderInput,
    Config: NumberConfig,
    toEnv: (_s, v) => String(v ?? ''),
  },

  bool: {
    id: 'bool',
    label: 'Toggle',
    blurb: 'On/off switch, passed as true/false',
    icon: ToggleLeft,
    yamlKeys: [],
    seed: () => ({ default: false }),
    emptyValue: () => false,
    Input: ToggleInput,
    toEnv: (_s, v) => (v ? 'true' : 'false'),
  },

  select: {
    id: 'select',
    label: 'Choice',
    blurb: 'Pick one of a fixed set',
    icon: List,
    yamlKeys: ['options'],
    seed: () => ({
      options: [
        { value: 'dev', label: 'Development', tone: 'success' as const },
        { value: 'prod', label: 'Production', tone: 'danger' as const },
      ],
      default: 'dev',
    }),
    emptyValue: () => '',
    Input: SelectInput,
    Config: SelectConfig,
    validate: (spec, value) => {
      const s = String(value ?? '');
      if (!s) return null;
      return spec.options?.some((o) => o.value === s) ? null : `"${s}" is not one of the options`;
    },
    toEnv: (_s, v) => String(v ?? ''),
  },

  multiselect: {
    id: 'multiselect',
    label: 'Multi-choice',
    blurb: 'Pick several; joined with commas',
    icon: ListChecks,
    yamlKeys: ['options', 'minSelected', 'maxSelected'],
    seed: () => ({
      options: [
        { value: 'eu-central' },
        { value: 'us-east' },
      ],
      default: [] as string[],
    }),
    emptyValue: () => [],
    Input: MultiSelectInput,
    Config: MultiSelectConfig,
    validate: (spec, value) => {
      const list = Array.isArray(value) ? value : [];
      if (spec.minSelected && list.length < spec.minSelected) {
        return `Choose at least ${spec.minSelected}`;
      }
      if (spec.maxSelected && list.length > spec.maxSelected) {
        return `Choose at most ${spec.maxSelected}`;
      }
      return null;
    },
    toEnv: (_s, v) => (Array.isArray(v) ? v.join(',') : String(v ?? '')),
  },

  date: {
    id: 'date',
    label: 'Date',
    blurb: 'Calendar picker, ISO output',
    icon: Calendar,
    yamlKeys: [],
    seed: () => ({ default: '' }),
    emptyValue: () => '',
    Input: DateInput,
    validate: (_spec, value) => {
      const s = String(value ?? '');
      return !s || ISO_DATE.test(s) ? null : 'Must be YYYY-MM-DD';
    },
    toEnv: (_s, v) => String(v ?? ''),
  },

  datetime: {
    id: 'datetime',
    label: 'Date & time',
    blurb: 'Calendar plus time, timezone-aware',
    icon: CalendarClock,
    yamlKeys: [],
    seed: () => ({ default: '' }),
    emptyValue: () => '',
    Input: DateTimeInput,
    toEnv: (_s, v) => String(v ?? ''),
  },

  secret: {
    id: 'secret',
    label: 'Secret',
    blurb: 'Resolved from the secret store, never echoed',
    icon: KeyRound,
    yamlKeys: ['secretRef'],
    seed: () => ({ secretRef: 'secrets/', required: true }),
    emptyValue: () => '',
    Input: SecretInput,
    Config: SecretConfig,
    toEnv: () => '••••••••',
  },

  file: {
    id: 'file',
    label: 'File',
    blurb: 'Uploaded to object storage, passed as a key',
    icon: FileUp,
    yamlKeys: ['accept'],
    seed: () => ({ default: '' }),
    emptyValue: () => '',
    Input: FileInput,
    Config: FileConfig,
    toEnv: (_s, v) => (v ? `s3://lute-uploads/${v}` : ''),
  },
};

/** Palette order — roughly by how often a job needs each. */
export const TYPE_ORDER: ParamTypeId[] = [
  'string',
  'select',
  'bool',
  'number',
  'multiselect',
  'date',
  'datetime',
  'secret',
  'slider',
  'file',
];

export function typeDef(id: ParamTypeId): ParamTypeDef {
  return PARAM_TYPES[id] ?? PARAM_TYPES.string;
}

/** The value a fresh form starts from: the declared default, else the type's empty. */
export function initialValue(spec: ParamSpec): ParamValue {
  return spec.default !== undefined ? spec.default : typeDef(spec.type).emptyValue(spec);
}

export function initialValues(specs: ParamSpec[]): Record<string, ParamValue> {
  return Object.fromEntries(specs.map((s) => [s.name, initialValue(s)]));
}

function isEmpty(value: ParamValue): boolean {
  if (Array.isArray(value)) return value.length === 0;
  return value === '' || value === undefined || value === null;
}

/**
 * Mirrors what Core does on trigger: same schema, same rules, so a payload the
 * panel accepts is a payload the API accepts (PRODUCT.md §5).
 */
export function validateAll(
  specs: ParamSpec[],
  values: Record<string, ParamValue>
): Record<string, string> {
  const errors: Record<string, string> = {};
  for (const spec of specs) {
    const value = values[spec.name];
    // Secrets are resolved server-side; the form never holds one to check.
    if (spec.type === 'secret') continue;
    if (spec.required && isEmpty(value)) {
      errors[spec.name] = 'Required';
      continue;
    }
    if (isEmpty(value)) continue;
    const msg = typeDef(spec.type).validate?.(spec, value);
    if (msg) errors[spec.name] = msg;
  }
  return errors;
}

/** The env block as the container will receive it. */
export function toEnvPairs(
  specs: ParamSpec[],
  values: Record<string, ParamValue>
): { key: string; value: string; masked: boolean }[] {
  return specs.map((spec) => ({
    key: spec.env,
    value: typeDef(spec.type).toEnv(spec, values[spec.name] ?? ''),
    masked: spec.type === 'secret',
  }));
}
