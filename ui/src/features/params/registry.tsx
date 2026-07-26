/**
 * The input-type registry.
 *
 * Adding a type means adding one entry here (plus its control and, if it needs
 * one, its config editor) — and the matching case in
 * `api/internal/jobdefs/validate.go` plus its name in `KnownTypes`, since the
 * server is the one that finally accepts or rejects a payload. Nothing else in
 * the UI enumerates types: the form renderer, the schema editor, the
 * validator, the YAML writer and the env preview all go through this table.
 */
import {
  Calendar,
  CalendarClock,
  Hash,
  KeyRound,
  List,
  ListChecks,
  ToggleLeft,
  Type,
} from 'lucide-react';
import {
  DateInput,
  DateTimeInput,
  MultiSelectInput,
  NumberInput,
  SecretInput,
  SelectInput,
  TextInput,
  ToggleInput,
} from './controls';
import { SecretConfig, SelectConfig } from './configs';
import type { ParamTypeDef } from './types';
import type { ParameterField, ParameterType, ParameterValue } from '@/types/jobs';

const ISO_DATE = /^\d{4}-\d{2}-\d{2}$/;

export const PARAM_TYPES: Record<ParameterType, ParamTypeDef> = {
  string: {
    id: 'string',
    label: 'Text',
    blurb: 'Free text',
    icon: Type,
    yamlKeys: [],
    seed: () => ({ default: '' }),
    emptyValue: () => '',
    Input: TextInput,
    toEnv: (_f, v) => String(v ?? ''),
    fromEnv: (_f, raw) => raw,
  },

  number: {
    id: 'number',
    label: 'Number',
    blurb: 'Numeric input',
    icon: Hash,
    yamlKeys: [],
    seed: () => ({ default: 0 }),
    emptyValue: () => '',
    Input: NumberInput,
    validate: (_f, value) => {
      if (value === '') return null; // emptiness is the `required` check's business
      return Number.isFinite(Number(value)) ? null : 'Must be a number';
    },
    toEnv: (_f, v) => String(v ?? ''),
    fromEnv: (_f, raw) => (raw === '' ? '' : Number(raw)),
  },

  bool: {
    id: 'bool',
    label: 'Toggle',
    blurb: 'On/off, passed as true/false',
    icon: ToggleLeft,
    yamlKeys: [],
    seed: () => ({ default: false }),
    emptyValue: () => false,
    Input: ToggleInput,
    toEnv: (_f, v) => (v ? 'true' : 'false'),
    fromEnv: (_f, raw) => raw === 'true',
  },

  select: {
    id: 'select',
    label: 'Choice',
    blurb: 'Pick one of a fixed set',
    icon: List,
    yamlKeys: ['options'],
    seed: () => ({ options: [{ value: 'dev', label: 'Development' }], default: 'dev' }),
    emptyValue: () => '',
    Input: SelectInput,
    Config: SelectConfig,
    validate: (field, value) => {
      const s = String(value ?? '');
      if (!s) return null;
      return field.options?.some((o) => o.value === s)
        ? null
        : 'Must be one of the allowed options';
    },
    toEnv: (_f, v) => String(v ?? ''),
    fromEnv: (_f, raw) => raw,
  },

  multiselect: {
    id: 'multiselect',
    label: 'Multi-choice',
    blurb: 'Pick several; joined with commas',
    icon: ListChecks,
    yamlKeys: ['options'],
    seed: () => ({ options: [{ value: 'one', label: 'One' }], default: [] as string[] }),
    emptyValue: () => [],
    Input: MultiSelectInput,
    Config: SelectConfig,
    validate: (field, value) => {
      const list = Array.isArray(value) ? value : [];
      const bad = list.find((v) => !field.options?.some((o) => o.value === v));
      return bad ? `"${bad}" is not an allowed option` : null;
    },
    toEnv: (_f, v) => (Array.isArray(v) ? v.join(',') : String(v ?? '')),
    // Server-side the value is a comma-joined string; split it back apart.
    fromEnv: (_f, raw) => (raw ? raw.split(',') : []),
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
    validate: (_f, value) => {
      const s = String(value ?? '');
      return !s || ISO_DATE.test(s) ? null : 'Must be a YYYY-MM-DD date';
    },
    toEnv: (_f, v) => String(v ?? ''),
    fromEnv: (_f, raw) => raw,
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
    validate: (_f, value) => {
      const s = String(value ?? '');
      if (!s) return null;
      // The server accepts RFC-3339 or a plain date; mirror both.
      return ISO_DATE.test(s) || !Number.isNaN(Date.parse(s))
        ? null
        : 'Must be an ISO-8601 datetime';
    },
    toEnv: (_f, v) => String(v ?? ''),
    fromEnv: (_f, raw) => raw,
  },

  secret: {
    id: 'secret',
    label: 'Secret',
    blurb: 'Resolved from the secret store, never echoed',
    icon: KeyRound,
    yamlKeys: ['secretRef'],
    seed: () => ({ secretRef: 'secrets/', required: true, default: undefined }),
    emptyValue: () => '',
    Input: SecretInput,
    Config: SecretConfig,
    toEnv: () => '',
    fromEnv: () => '',
  },
};

/** Palette order — roughly by how often a job needs each. */
export const TYPE_ORDER: ParameterType[] = [
  'string',
  'select',
  'bool',
  'number',
  'multiselect',
  'date',
  'datetime',
  'secret',
];

export function typeDef(id: ParameterType): ParamTypeDef {
  return PARAM_TYPES[id] ?? PARAM_TYPES.string;
}

/** The value a fresh form starts from: the declared default, else the empty. */
export function initialValue(field: ParameterField): ParameterValue {
  return field.default !== undefined ? field.default : typeDef(field.type).emptyValue(field);
}

export function initialValues(fields: ParameterField[]): Record<string, ParameterValue> {
  return Object.fromEntries(fields.map((f) => [f.name, initialValue(f)]));
}

/**
 * Rebuilds form values from a previous build's stored env map. Fields the
 * build did not carry fall back to their default, so a schema that has since
 * gained a parameter still yields a complete form.
 */
export function valuesFromEnv(
  fields: ParameterField[],
  env: Record<string, string> | undefined
): Record<string, ParameterValue> {
  return Object.fromEntries(
    fields.map((f) => {
      const raw = env?.[f.envVar];
      if (raw === undefined || f.type === 'secret') return [f.name, initialValue(f)];
      return [f.name, typeDef(f.type).fromEnv(f, raw)];
    })
  );
}

export function isEmpty(value: ParameterValue | undefined): boolean {
  if (Array.isArray(value)) return value.length === 0;
  return value === '' || value === undefined || value === null;
}

/**
 * Mirrors `Validate` in api/internal/jobdefs/validate.go: same schema, same
 * rules, so a payload the panel accepts is a payload the API accepts. The
 * server remains the authority — this only saves a round trip.
 */
export function validateAll(
  fields: ParameterField[],
  values: Record<string, ParameterValue>
): Record<string, string> {
  const errors: Record<string, string> = {};
  for (const field of fields) {
    // Secrets resolve worker-side from their secretRef; the form never holds one.
    if (field.type === 'secret') continue;

    const value = values[field.name];
    // The server falls back to the declared default before deciding a field is
    // missing, so an empty input with a default is not an error here either.
    if (isEmpty(value) && !isEmpty(field.default as ParameterValue)) continue;
    if (isEmpty(value)) {
      if (field.required) errors[field.name] = 'required';
      continue;
    }
    const msg = typeDef(field.type).validate?.(field, value);
    if (msg) errors[field.name] = msg;
  }
  return errors;
}

/** The env block as the container will receive it. */
export function toEnvPairs(
  fields: ParameterField[],
  values: Record<string, ParameterValue>
): { key: string; value: string; masked: boolean }[] {
  return fields.map((field) => ({
    key: field.envVar,
    value: typeDef(field.type).toEnv(field, values[field.name] ?? ''),
    masked: field.type === 'secret',
  }));
}
