/**
 * Job-definition service — talks to the Core API.
 *
 * Endpoints (see api/internal/jobdefs): job definitions are Git-managed and
 * synced into Postgres; the parameter schema both renders the trigger UI and
 * is validated server-side on trigger.
 */

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
    `/api/v1/job-definitions/${encodeURIComponent(slug)}/builds`
  );
  return res.builds ?? [];
}

/**
 * Triggers a build of a Git-managed definition.
 *
 * `parameters` is the schema the panel actually rendered. Sending it makes the
 * server validate against what the user saw: when it differs from the committed
 * definition the build is recorded as ad-hoc, and it is rejected with 409
 * `adhoc_builds_disabled` if the operator has turned ad-hoc builds off. Omitting
 * it silently dropped values for any parameter added in the workbench.
 */
export function triggerBuild(
  slug: string,
  values: ParameterValues,
  parameters?: ParameterField[]
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

/**
 * Saves a panel-authored template. Stored with origin=panel so the Git sync
 * neither rewrites nor prunes it.
 */
export function createJob(template: NewJobTemplate): Promise<JobDefinition> {
  return apiClient.post<JobDefinition>('/api/v1/job-definitions', template);
}

/**
 * Saves edits to a panel-authored definition. Git-managed ones are refused with
 * 409 `git_managed` — a sync would overwrite the change anyway.
 */
export function updateJob(slug: string, template: NewJobTemplate): Promise<JobDefinition> {
  return apiClient.put<JobDefinition>(
    `/api/v1/job-definitions/${encodeURIComponent(slug)}`,
    template
  );
}

