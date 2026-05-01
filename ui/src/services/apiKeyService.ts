import { apiClient } from './api';

export interface APIKeySummary {
    id: string;
    name: string;
    prefix: string;
    created_at: string;
    last_used_at?: string;
    revoked: boolean;
}

export interface CreateAPIKeyResponse {
    id: string;
    name: string;
    prefix: string;
    token: string;
    created_at: string;
}

export const apiKeyService = {
    list: async (): Promise<{ api_keys: APIKeySummary[] }> => {
        return apiClient.get('/api/v1/api-keys');
    },
    create: async (name: string): Promise<CreateAPIKeyResponse> => {
        return apiClient.post('/api/v1/api-keys', { name });
    },
    revoke: async (id: string): Promise<void> => {
        await apiClient.delete(`/api/v1/api-keys/${id}`);
    },
};
