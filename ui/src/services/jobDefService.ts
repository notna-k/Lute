// Job-definition endpoints (api/internal/jobdefs).

import { apiClient } from './api';
import type { Build, JobDefinition, ParameterField, ParameterValues } from '@/types/jobs';

export async function listJobs(): Promise<JobDefinition[]> {
  const res = await apiClient.get<{ jobs: JobDefinition[] }>('/api/v1/job-definitions');
  return res.jobs ?? [];
}

export function getJob(slug: string): Promise<JobDefinition> {
  return apiClient.get<JobDefinition>(`/api/v1/job-definitions/${encodeURIComponent(slug)}`);
}

export async function listBuilds(slug: string): Promise<Build[]> {
  const res = await apiClient.get<{ builds: Build[] }>(
    `/api/v1/job-definitions/${encodeURIComponent(slug)}/builds`,
  );
  return res.builds ?? [];
}

/**
 * Triggers a build. `parameters` is the schema the panel rendered: if it differs from the
 * committed one the build is ad-hoc, and gets 409 `conflict` when those are off.
 */
export function triggerBuild(
  slug: string,
  values: ParameterValues,
  parameters?: ParameterField[],
): Promise<Build> {
  return apiClient.post<Build>(`/api/v1/job-definitions/${encodeURIComponent(slug)}/trigger`, {
    values,
    parameters,
  });
}

/** A template authored in the panel and saved as a definition. */
export interface NewJobTemplate {
  name: string;
  description?: string;
  queue: string;
  runtime: string;
  command: string;
  sourceRepo?: string;
  labelSelector?: Record<string, string>;
  parameters: ParameterField[];
}

/** Saves a panel-authored template; it shows as "not in Git" until committed. */
export function createJob(template: NewJobTemplate): Promise<JobDefinition> {
  return apiClient.post<JobDefinition>('/api/v1/job-definitions', template);
}

/** Saves edits; on a Git definition they stand until its file changes in Git. */
export function updateJob(slug: string, template: NewJobTemplate): Promise<JobDefinition> {
  return apiClient.put<JobDefinition>(
    `/api/v1/job-definitions/${encodeURIComponent(slug)}`,
    template,
  );
}

/** What one sync did (api/internal/jobdefs/sync.go SyncResult). */
export interface SyncResult {
  added: number;
  updated: number;
  unchanged: number;
  detached: number;
  pruned: number;
  /** Files or documents that failed to parse. */
  skipped: string[];
}

/** Reconciles definitions with the Git source now, rather than on restart. */
export function syncJobs(): Promise<SyncResult> {
  return apiClient.post<SyncResult>('/api/v1/job-definitions/sync', {});
}

/** Every definition as one multi-document YAML stream, ready to commit. */
export async function exportJobs(): Promise<string> {
  const res = await apiClient.get<{ yaml: string }>('/api/v1/job-definitions/export');
  return res.yaml;
}

/** One definition's YAML, as it would be committed. */
export async function exportJob(slug: string): Promise<string> {
  const res = await apiClient.get<{ yaml: string }>(
    `/api/v1/job-definitions/${encodeURIComponent(slug)}/yaml`,
  );
  return res.yaml;
}

/** Every definition as a zip of one YAML file per job, at its path in Git. */
export function exportJobsZip(): Promise<Blob> {
  return apiClient.getBlob('/api/v1/job-definitions/export.zip');
}

/** Discards panel edits, restoring what Git last said. */
export function revertJob(slug: string): Promise<JobDefinition> {
  return apiClient.post<JobDefinition>(
    `/api/v1/job-definitions/${encodeURIComponent(slug)}/revert`,
    {},
  );
}
