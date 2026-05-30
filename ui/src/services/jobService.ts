import { apiClient } from './api';

export interface Job {
    id: string;
    queue: string;
    type: string;
    payload: unknown;
    status: 'pending' | 'running' | 'done' | 'dead' | 'cancelled';
    attempts: number;
    max_retries: number;
    timeout_sec: number;
    error?: string;
    worker_id?: string;
    enqueued_at: number;
    started_at?: number;
    done_at?: number;
    meta?: Record<string, string>;
}

export interface EnqueueRequest {
    queue: string;
    type: string;
    payload?: unknown;
    priority?: number;
    delay_ms?: number;
    max_retries?: number;
    timeout_sec?: number;
    selector?: Record<string, string>;
}

export interface EnqueueResponse {
    job_id: string;
    status: string;
    message: string;
}

export interface ListJobsResponse {
    jobs: Job[];
    count: number;
}

export interface JobLogsResponse {
    lines: string[];
    direction: string;
    has_more: boolean;
    file_size: number;
    next_cursor?: string;
    error?: string;
}

export const jobService = {
    listJobs: async (queueName: string, params?: { offset?: number; limit?: number }): Promise<ListJobsResponse> => {
        const qs = new URLSearchParams();
        if (params?.offset !== undefined) qs.set('offset', String(params.offset));
        if (params?.limit !== undefined) qs.set('limit', String(params.limit));
        const query = qs.toString() ? `?${qs.toString()}` : '';
        return apiClient.get<ListJobsResponse>(`/api/v1/queues/${queueName}/jobs${query}`);
    },

    listQueues: async (): Promise<{ queues: { name: string; depth: number }[] }> => {
        return apiClient.get('/api/v1/queues');
    },

    getJob: async (id: string): Promise<Job> => {
        return apiClient.get<Job>(`/api/v1/jobs/${id}`);
    },

    enqueueJob: async (data: EnqueueRequest): Promise<EnqueueResponse> => {
        return apiClient.post<EnqueueResponse>('/api/v1/jobs', data);
    },

    retryJob: async (id: string): Promise<{ message: string; job_id: string }> => {
        return apiClient.post(`/api/v1/jobs/${id}/retry`);
    },

    cancelJob: async (id: string): Promise<{ message: string }> => {
        return apiClient.delete(`/api/v1/jobs/${id}`);
    },

    getJobLogs: async (
        id: string,
        params?: { direction?: 'tail' | 'head'; limit?: number; cursor?: string }
    ): Promise<JobLogsResponse> => {
        const qs = new URLSearchParams();
        qs.set('direction', params?.direction ?? 'tail');
        qs.set('limit', String(params?.limit ?? 200));
        if (params?.cursor) qs.set('cursor', params.cursor);
        return apiClient.get<JobLogsResponse>(`/api/v1/jobs/${id}/logs?${qs.toString()}`);
    },
};
