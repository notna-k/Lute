/**
 * The parameter-type contract.
 *
 * PRODUCT.md §5 makes the parameter schema the crown jewel: one definition
 * drives both the trigger UI and server-side validation. This module is built
 * so that *adding an input type is one registry entry plus a control* — no
 * `switch` in a form component, no consumer to update.
 *
 * The schema shape itself is `ParameterField` from `@/types/jobs`, which
 * mirrors `models.ParameterField` on the server. Deliberately not extended
 * here: a key the server does not understand would be dropped at sync time,
 * so the UI would promise a constraint nothing enforces.
 */
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

/**
 * Everything the app knows about one input type, in one place: how it is
 * authored, rendered, validated, serialised, and handed to the container.
 */
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
  /**
   * Returns an error message, or null when the value satisfies the field.
   * Must mirror `coerce` in api/internal/jobdefs/validate.go — the panel is a
   * fast path for the same rules, never a different set of them.
   */
  validate?: (field: ParameterField, value: ParameterValue) => string | null;
  /** How the value reaches the container (the stored `request_params` form). */
  toEnv: (field: ParameterField, value: ParameterValue) => string;
  /**
   * The inverse of `toEnv`: turns a stored env string back into a form value,
   * so a build can be prefilled from a previous one.
   */
  fromEnv: (field: ParameterField, raw: string) => ParameterValue;
}
