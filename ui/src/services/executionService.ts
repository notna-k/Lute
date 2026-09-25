import { apiClient } from './api';

interface JobExecution {
  id: string;
  created_at: string;
  updated_at: string;
  job_id: string;
  worker_id: string;
  queue: string;
  type: string;
  success: boolean;
  error?: string;
  elapsed_ms: number;
  log_file?: string;
  execution_log_file?: string;
  finished_at: string;
}

export type ExecutionSort = 'finished_at_desc' | 'finished_at_asc' | 'elapsed_desc' | 'elapsed_asc';

export interface ListExecutionsParams {
  /** Any of these queues; empty or omitted means all of them. */
  queues?: string[];
  types?: string[];
  status?: '' | 'success' | 'failed';
  /** Free text over the run id, the worker id and the error message. */
  search?: string;
  offset?: number;
  limit?: number;
  sort?: ExecutionSort;
}

export interface ListExecutionsResponse {
  executions: JobExecution[];
  total: number;
  offset: number;
  limit: number;
}

export interface ExecutionFilterOptions {
  queues: string[];
  types: string[];
}

export const executionService = {
  list: async (params?: ListExecutionsParams): Promise<ListExecutionsResponse> => {
    const qs = new URLSearchParams();
    // Repeated keys, so a queue name containing a comma survives.
    params?.queues?.forEach((q) => q && qs.append('queue', q));
    params?.types?.forEach((t) => t && qs.append('type', t));
    if (params?.status) qs.set('status', params.status);
    if (params?.search?.trim()) qs.set('q', params.search.trim());
    if (params?.offset !== undefined) qs.set('offset', String(params.offset));
    if (params?.limit !== undefined) qs.set('limit', String(params.limit));
    if (params?.sort) qs.set('sort', params.sort);
    const q = qs.toString();
    return apiClient.get<ListExecutionsResponse>(`/api/v1/executions${q ? `?${q}` : ''}`);
  },

  filterOptions: async (): Promise<ExecutionFilterOptions> => {
    return apiClient.get<ExecutionFilterOptions>('/api/v1/executions/filter-options');
  },
};
