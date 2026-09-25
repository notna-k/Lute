// Job definitions and their parameter schema, as the Core API serves them.

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

interface JobSource {
  repo: string;
  path: string;
  commit: string;
}

/**
 * Relation to Git (api/internal/db/models/job_definition.go): `modified` stands until the file
 * changes, `manual` was created in the panel, `removed` is kept because pruning is off.
 */
export type GitState = 'synced' | 'modified' | 'manual' | 'removed';

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
  gitState: GitState;
  /** Success ratio over the trailing 30 days, 0..1. */
  successRate: number;
  medianDurationMs: number;
  /** Newest build of this job, so a list row can show what it is doing now. */
  lastBuild?: Build;
  /** Trailing build statuses, oldest first, for the history strip. */
  recent?: BuildStatus[];
}

export type BuildStatus = 'running' | 'passed' | 'failed' | 'queued' | 'aborted';

export interface Build {
  id: string;
  /** Full run identifier — use this to address the build in the runs API. */
  runId?: string;
  /** Queue job identifier — use this for the queue and log endpoints (/jobs/:id/...). */
  jobId?: string;
  jobSlug: string;
  status: BuildStatus;
  environment?: string;
  startedAt: number;
  durationMs?: number;
  /** Resolved values keyed by env var (never secrets), used to prefill a new build. */
  params?: Record<string, string>;
  /** True when this build ran a panel-edited schema, not the committed one. */
  adHoc?: boolean;
}
