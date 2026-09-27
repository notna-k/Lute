/** The container engine a worker runs jobs on, as its agent reported it. */
export interface WorkerEngine {
  kind: string;
  version: string;
  rootless: boolean;
  memory_limit: boolean;
  cpu_limit: boolean;
  pids_limit: boolean;
}

/** Lets agents enrol themselves; the plaintext is only in the create response. */
export interface RegistrationToken {
  id: string;
  name: string;
  prefix: string;
  created_at: string;
  last_used_at?: string;
  revoked_at?: string;
}

// Worker row from API (registered agent / compute node).
export interface Worker {
  id: string;
  user_id: string;
  name: string;
  description?: string;
  status: 'pending' | 'registered' | 'alive' | 'dead' | 'deleting';
  agent_version?: string;
  /** The agent is older than core: pull its image again. */
  outdated?: boolean;
  engine?: WorkerEngine;
  last_seen?: string;
  metadata?: Record<string, unknown>;
  /** Canonical keys: cpu_load, mem_usage_mb, disk_used_gb, disk_total_gb (numbers). */
  metrics?: Record<string, string | number>;
  /** Operator-assigned key-value labels for routing and filtering. */
  labels?: Record<string, string>;
  created_at: string;
  updated_at: string;
}
