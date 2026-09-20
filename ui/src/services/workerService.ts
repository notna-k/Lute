import { apiClient } from './api';
import { Worker } from '../types';

export interface CreateWorkerRequest {
    name: string;
    description?: string;
    metadata?: Record<string, unknown>;
}

export interface UpdateWorkerRequest {
    name?: string;
    description?: string;
    status?: string;
    metadata?: Record<string, unknown>;
}

export const workerService = {
    getUserWorkers: async (): Promise<Worker[]> => {
        const data = await apiClient.get<Worker[] | null>('/api/v1/workers');
        return data ?? [];
    },

    getWorker: async (id: string): Promise<Worker> => {
        return apiClient.get<Worker>(`/api/v1/workers/${id}`);
    },

    createWorker: async (data: CreateWorkerRequest): Promise<Worker> => {
        return apiClient.post<Worker>('/api/v1/workers', data);
    },

    updateWorker: async (id: string, data: UpdateWorkerRequest): Promise<Worker> => {
        return apiClient.put<Worker>(`/api/v1/workers/${id}`, data);
    },

    reEnableWorker: async (id: string): Promise<Worker> => {
        return apiClient.post<Worker>(`/api/v1/workers/${id}/re-enable`);
    },

    deleteWorker: async (id: string): Promise<void> => {
        return apiClient.delete<void>(`/api/v1/workers/${id}`);
    },

    updateLabels: async (id: string, labels: Record<string, string>): Promise<Worker> => {
        return apiClient.patch<Worker>(`/api/v1/workers/${id}/labels`, { labels });
    },
};
