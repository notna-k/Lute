// The parameter-type contract: adding an input type is one registry entry plus a control.
import type { ComponentType } from 'react';
import type { LucideIcon } from 'lucide-react';
import type { ParameterField, ParameterType, ParameterValue } from '@/types/jobs';

export interface ParamInputProps {
  field: ParameterField;
  value: ParameterValue;
  onChange: (value: ParameterValue) => void;
  invalid?: boolean;
  /** Set when the field is displayed for preview rather than entry. */
  disabled?: boolean;
}

export interface ParamConfigProps {
  field: ParameterField;
  onChange: (patch: Partial<ParameterField>) => void;
}

/** Everything the app knows about one input type, from authoring to the container env. */
export interface ParamTypeDef {
  id: ParameterType;
  label: string;
  /** One-line description, shown when picking a type. */
  blurb: string;
  icon: LucideIcon;
  /** Keys beyond the common set that this type writes to YAML, in order. */
  yamlKeys: (keyof ParameterField)[];
  /** Partial field seeded when an author switches to or adds this type. */
  seed: () => Partial<ParameterField>;
  /** Value used when the field declares no default. */
  emptyValue: (field: ParameterField) => ParameterValue;
  /** The control someone triggering a build interacts with. */
  Input: ComponentType<ParamInputProps>;
  /** Type-specific controls in the schema editor, beyond the common ones. */
  Config?: ComponentType<ParamConfigProps>;
  /** Error message or null. Must mirror `coerce` in api/internal/jobdefs/validate.go. */
  validate?: (field: ParameterField, value: ParameterValue) => string | null;
  /** How the value reaches the container (the stored `request_params` form). */
  toEnv: (field: ParameterField, value: ParameterValue) => string;
  /** Inverse of `toEnv`, so a build can be prefilled from a previous one. */
  fromEnv: (field: ParameterField, raw: string) => ParameterValue;
}
