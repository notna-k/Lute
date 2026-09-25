import { authBridge } from '../contexts/AuthContext';
import { API_URL } from './apiBase';

/**
 * An error response from the API: `{"error": {"code", "message", "fields"}}`.
 * `code` is one of a small fixed set (e.g. `not_found`, `conflict`,
 * `validation_failed`); `fields` maps an input to what is wrong with it.
 */
export class ApiError extends Error {
  readonly status: number;
  readonly code: string;
  readonly fields?: Record<string, string>;

  constructor(message: string, status: number, code: string, fields?: Record<string, string>) {
    super(message);
    this.name = 'ApiError';
    this.status = status;
    this.code = code;
    this.fields = fields;
  }
}

interface ErrorBody {
  error?: { code?: string; message?: string; fields?: Record<string, string> };
}

async function toApiError(response: Response): Promise<ApiError> {
  const body = (await response.json().catch(() => ({}))) as ErrorBody;
  const err = body.error ?? {};
  return new ApiError(
    err.message || response.statusText || `HTTP ${response.status}`,
    response.status,
    err.code ?? '',
    err.fields,
  );
}

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

    const token = authBridge.getAccessToken();
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

    if (!response.ok) throw await toApiError(response);
    return response.json() as Promise<T>;
  }

  async get<T>(endpoint: string, init?: RequestInit): Promise<T> {
    return this.request<T>(endpoint, { method: 'GET', ...init });
  }

  /**
   * A GET whose body is a file rather than JSON — the zip export. It repeats
   * `request`'s auth rather than sharing it because the whole point is to not
   * parse the response as JSON.
   */
  async getBlob(endpoint: string): Promise<Blob> {
    const send = (token: string | null) =>
      fetch(`${this.baseURL}${endpoint}`, {
        method: 'GET',
        credentials: 'include',
        headers: token ? { Authorization: `Bearer ${token}` } : {},
      });

    let response = await send(authBridge.getAccessToken());
    if (response.status === 401) {
      const refreshed = await authBridge.refresh();
      if (refreshed) response = await send(refreshed);
      else await authBridge.signOut();
    }
    if (!response.ok) throw await toApiError(response);
    return response.blob();
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

  async patch<T>(endpoint: string, data?: unknown): Promise<T> {
    return this.request<T>(endpoint, {
      method: 'PATCH',
      body: data ? JSON.stringify(data) : undefined,
    });
  }

  async delete<T>(endpoint: string): Promise<T> {
    return this.request<T>(endpoint, { method: 'DELETE' });
  }
}

export const apiClient = new ApiClient(API_URL);
