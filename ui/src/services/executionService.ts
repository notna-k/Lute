import { apiClient } from './api';

export interface JobExecution {
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

export interface ListExecutionsParams {
    queue?: string;
    type?: string;
    status?: '' | 'success' | 'failed';
    offset?: number;
    limit?: number;
    sort?: 'finished_at_desc' | 'finished_at_asc';
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
        if (params?.queue) qs.set('queue', params.queue);
        if (params?.type) qs.set('type', params.type);
        if (params?.status) qs.set('status', params.status);
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
