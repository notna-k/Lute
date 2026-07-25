/**
 * Job definitions and their parameter schema.
 *
 * Mirrors the model in PRODUCT.md: a Job is a reusable, Git-managed definition
 * whose `parameters` schema both renders the trigger UI and validates payloads
 * server-side. Types here are the UI-facing shape the backend will serve.
 */

export type ParameterType =
  | 'string'
  | 'number'
  | 'bool'
  | 'select'
  | 'multiselect'
  | 'date'
  | 'datetime'
  | 'secret';

export interface ParameterOption {
  value: string;
  label: string;
  /** Optional descriptive line shown under the label in rich selects. */
  hint?: string;
  /** Optional tone tag (e.g. an environment: dev / staging / prod). */
  tone?: 'neutral' | 'success' | 'warning' | 'danger';
}

export interface ParameterField {
  name: string;
  type: ParameterType;
  label: string;
  /** Environment variable the value is passed to the container as. */
  envVar: string;
  description?: string;
  required?: boolean;
  default?: string | number | boolean | string[];
  options?: ParameterOption[];
  /** For `secret`: where the value is resolved from (never echoed). */
  secretRef?: string;
}

export type ParameterValue = string | number | boolean | string[];
export type ParameterValues = Record<string, ParameterValue>;

export interface JobSource {
  repo: string;
  path: string;
  commit: string;
  inSync: boolean;
}

export interface JobDefinition {
  slug: string;
  name: string;
  description: string;
  queue: string;
  labelSelector: Record<string, string>;
  runtime: string;
  command: string;
  source: JobSource;
  parameters: ParameterField[];
  /** Success ratio over the trailing 30 days, 0..1. */
  successRate: number;
  medianDurationMs: number;
}

export type BuildStatus = 'running' | 'passed' | 'failed' | 'queued';

export interface Build {
  id: string;
  /** Full run identifier — use this to address the build in APIs. */
  runId?: string;
  jobSlug: string;
  status: BuildStatus;
  environment?: string;
  startedAt: number;
  durationMs?: number;
  /**
   * Resolved values this build ran with, keyed by env var (never secrets).
   * Lets the panel prefill a new build from a previous one.
   */
  params?: Record<string, string>;
}
