export interface User {
  uid: string;
  email: string | null;
  displayName: string | null;
  photoURL: string | null;
}

// Machine interface matching backend API response
export interface Machine {
  id: string;
  user_id: string;
  name: string;
  description?: string;
  status: 'running' | 'stopped' | 'paused' | 'pending' | 'alive' | 'dead';
  is_public: boolean;
  agent_ip?: string;
  agent_version?: string;
  last_seen?: string;
  metadata?: Record<string, unknown>;
  metrics?: Record<string, string>;
  created_at: string;
  updated_at: string;
}

// Legacy VM interface for backward compatibility (can be removed later)
export interface VM {
  id: string;
  name: string;
  status: 'running' | 'stopped' | 'paused';
  cpu: number;
  memory: number;
  disk: number;
  ownerId?: string;
  isPublic?: boolean;
  createdAt: string;
  updatedAt: string;
}

