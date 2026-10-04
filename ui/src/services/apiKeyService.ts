import { apiClient } from './api';

/** Account keys act as the user who made them; service keys belong to the instance. */
export type APIKeyScope = 'account' | 'service';

export interface APIKeySummary {
  id: string;
  name: string;
  scope: APIKeyScope;
  prefix: string;
  created_by?: string;
  created_by_email?: string;
  created_at: string;
  last_used_at?: string;
  revoked: boolean;
}

export interface CreateAPIKeyResponse {
  id: string;
  name: string;
  scope: APIKeyScope;
  prefix: string;
  token: string;
  created_at: string;
}

export const apiKeyService = {
  list: async (scope: APIKeyScope): Promise<{ api_keys: APIKeySummary[] }> => {
    return apiClient.get(`/api/v1/api-keys?scope=${scope}`);
  },
  create: async (name: string, scope: APIKeyScope): Promise<CreateAPIKeyResponse> => {
    return apiClient.post('/api/v1/api-keys', { name, scope });
  },
  revoke: async (id: string): Promise<void> => {
    await apiClient.delete(`/api/v1/api-keys/${id}`);
  },
};
