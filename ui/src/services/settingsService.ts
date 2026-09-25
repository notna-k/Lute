// Operator policy switches (api/internal/settings). Never job-definition config.

import { apiClient } from './api';

export interface Settings {
  /** Whether a schema that differs from Git (edited or panel-created) may be run. */
  allowAdhocBuilds: boolean;
  /** Whether a Git sync deletes definitions no YAML file defines, panel templates included. */
  pruneDefinitions: boolean;
}

export function getSettings(): Promise<Settings> {
  return apiClient.get<Settings>('/api/v1/settings');
}

export function updateSettings(patch: Partial<Settings>): Promise<Settings> {
  return apiClient.put<Settings>('/api/v1/settings', patch);
}
