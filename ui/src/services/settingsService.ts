/**
 * Operator settings — panel-managed policy switches (see api/internal/settings).
 *
 * These are deliberately not job-definition config: PRODUCT.md's canonical
 * serialization rule governs definitions, and nothing here may encode something
 * a definition could express.
 */

import { apiClient } from './api';

export interface Settings {
  /**
   * Whether a template whose schema differs from the definition in Git — edited
   * in the workbench, or created from scratch — may be run. Off means every
   * build must come from a committed definition.
   */
  allowAdhocBuilds: boolean;
}

export function getSettings(): Promise<Settings> {
  return apiClient.get<Settings>('/api/v1/settings');
}

export function updateSettings(patch: Partial<Settings>): Promise<Settings> {
  return apiClient.put<Settings>('/api/v1/settings', patch);
}
