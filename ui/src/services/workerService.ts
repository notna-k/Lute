import { apiClient } from './api';
import type { Worker } from '@/types';

export const workerService = {
  getUserWorkers: async (): Promise<Worker[]> => {
    const data = await apiClient.get<Worker[] | null>('/api/v1/workers');
    return data ?? [];
  },

  getWorker: async (id: string): Promise<Worker> => {
    return apiClient.get<Worker>(`/api/v1/workers/${id}`);
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
