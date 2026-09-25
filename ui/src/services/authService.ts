// Auth calls bypass apiClient: they produce the access token it attaches. The refresh token is an
// httpOnly cookie; the access token lives in memory in AuthContext.
import { API_URL as BASE } from './apiBase';

export interface AuthUser {
  id: string;
  email: string;
  display_name: string;
}

export interface LoginResponse {
  access_token: string;
  expires_in: number;
  token_type: string;
  user: AuthUser;
}

async function postJSON<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const body = (await res.json().catch(() => ({}))) as { error?: { message?: string } };
    throw new Error(body.error?.message || `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const login = (email: string, password: string) =>
  postJSON<LoginResponse>('/api/v1/auth/login', { email, password });

export const refresh = () => postJSON<LoginResponse>('/api/v1/auth/refresh');

export const logout = () => postJSON<{ ok: true }>('/api/v1/auth/logout');
