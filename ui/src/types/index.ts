export interface User {
  id: string;
  email: string;
  display_name: string;
}

// Worker row from API (registered agent / compute node).
export interface Worker {
  id: string;
  user_id: string;
  name: string;
  description?: string;
  status: 'running' | 'stopped' | 'paused' | 'pending' | 'alive' | 'dead';
  agent_ip?: string;
  agent_version?: string;
  last_seen?: string;
  metadata?: Record<string, unknown>;
  /** Canonical keys: cpu_load, mem_usage_mb, disk_used_gb, disk_total_gb (numbers). */
  metrics?: Record<string, string | number>;
  /** Operator-assigned key-value labels for routing and filtering. */
  labels?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

// Job selector: key-value pairs that a worker's labels must contain for the job to be dispatched.
export type JobSelector = Record<string, string>;

// Legacy VM interface for backward compatibility (can be removed later)
export interface VM {
  id: string;
  name: string;
  status: 'running' | 'stopped' | 'paused';
  cpu: number;
  memory: number;
  disk: number;
  ownerId?: string;
  createdAt: string;
  updatedAt: string;
}

