import { apiClient } from './api';
import type { RegistrationToken, Worker } from '@/types';

/** What the Add Worker dialog needs for its `docker run` command. */
export interface InstallInfo {
  server: string;
  image: string;
}

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

  /** "deleting" while a connected worker drains, "deleted" when it is gone at once. */
  deleteWorker: async (id: string): Promise<{ status: 'deleting' | 'deleted' }> => {
    return apiClient.delete(`/api/v1/workers/${id}`);
  },

  listTokens: async (): Promise<RegistrationToken[]> => {
    const data = await apiClient.get<{ tokens: RegistrationToken[] | null }>(
      '/api/v1/workers/tokens',
    );
    return data.tokens ?? [];
  },

  createToken: async (name: string): Promise<RegistrationToken & { token: string }> => {
    return apiClient.post('/api/v1/workers/tokens', { name });
  },

  revokeToken: async (id: string): Promise<void> => {
    await apiClient.delete(`/api/v1/workers/tokens/${id}`);
  },

  installInfo: async (): Promise<InstallInfo> => {
    return apiClient.get('/api/v1/workers/install');
  },

  updateLabels: async (id: string, labels: Record<string, string>): Promise<Worker> => {
    return apiClient.patch<Worker>(`/api/v1/workers/${id}/labels`, { labels });
  },
};
