import { authBridge } from '../contexts/AuthContext';

/** Empty string = same-origin (embedded UI + API). */
function resolveApiBaseURL(): string {
  const raw = import.meta.env.VITE_API_URL;
  if (raw === undefined || raw === null) {
    return 'http://localhost:8080';
  }
  const s = String(raw).trim();
  if (s === '') {
    return '';
  }
  return s.replace(/\/$/, '');
}

const API_URL = resolveApiBaseURL();

class ApiClient {
  private baseURL: string;

  constructor(baseURL: string) {
    this.baseURL = baseURL;
  }

  private buildHeaders(token: string | null, init?: RequestInit): Record<string, string> {
    const headers: Record<string, string> = { 'Content-Type': 'application/json' };
    if (init?.headers) {
      if (init.headers instanceof Headers) {
        init.headers.forEach((v, k) => (headers[k] = v));
      } else if (Array.isArray(init.headers)) {
        init.headers.forEach(([k, v]) => (headers[k] = v));
      } else {
        Object.assign(headers, init.headers);
      }
    }
    if (token) headers['Authorization'] = `Bearer ${token}`;
    return headers;
  }

  private async request<T>(endpoint: string, options: RequestInit = {}): Promise<T> {
    const send = async (token: string | null) =>
      fetch(`${this.baseURL}${endpoint}`, {
        ...options,
        credentials: 'include',
        headers: this.buildHeaders(token, options),
      });

    let token = authBridge.getAccessToken();
    let response = await send(token);

    if (response.status === 401 && endpoint !== '/api/v1/auth/refresh') {
      // Try a silent refresh once, then retry.
      const refreshed = await authBridge.refresh();
      if (refreshed) {
        response = await send(refreshed);
      } else {
        await authBridge.signOut();
      }
    }

    if (!response.ok) {
      const error = await response.json().catch(() => ({ error: response.statusText }));
      throw new Error(error.error || `HTTP error! status: ${response.status}`);
    }
    return response.json() as Promise<T>;
  }

  async get<T>(endpoint: string, init?: RequestInit): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET', ...init });
  }

  async post<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'POST',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async put<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PUT',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' });
  }
}

export const apiClient = new ApiClient(API_URL);
