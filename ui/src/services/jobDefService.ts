/**
 * Job-definition service — talks to the Core API.
 *
 * Endpoints (see api/internal/jobdefs): job definitions are Git-managed and
 * synced into Postgres; the parameter schema both renders the trigger UI and
 * is validated server-side on trigger.
 */

import { apiClient } from './api';
import type { Build, JobDefinition, ParameterValues } from '@/types/jobs';

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

/** Triggers a build. The server validates values against the parameter schema. */
export function triggerBuild(slug: string, values: ParameterValues): Promise<Build> {
  return apiClient.post<Build>(`/api/v1/job-definitions/${encodeURIComponent(slug)}/trigger`, {
    values,
  });
}
