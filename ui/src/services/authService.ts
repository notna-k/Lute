// Client-side auth helpers. The refresh token lives in an httpOnly cookie
// set by the API; the access token is held in memory by AuthContext.

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

function apiBaseURL(): string {
  const raw = import.meta.env.VITE_API_URL;
  if (raw === undefined || raw === null) return 'http://localhost:8080';
  const s = String(raw).trim();
  return s === '' ? '' : s.replace(/\/$/, '');
}

const BASE = apiBaseURL();

async function postJSON<T>(path: string, body?: unknown): Promise<T> {
  const res = await fetch(`${BASE}${path}`, {
    method: 'POST',
    credentials: 'include',
    headers: { 'Content-Type': 'application/json' },
    body: body ? JSON.stringify(body) : undefined,
  });
  if (!res.ok) {
    const err = (await res.json().catch(() => ({}))) as { error?: string };
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.json() as Promise<T>;
}

export const login = (email: string, password: string) =>
  postJSON<LoginResponse>('/api/v1/auth/login', { email, password });

export const refresh = () => postJSON<LoginResponse>('/api/v1/auth/refresh');

export const logout = () => postJSON<{ ok: true }>('/api/v1/auth/logout');

export const me = async (accessToken: string): Promise<AuthUser> => {
  const res = await fetch(`${BASE}/api/v1/auth/me`, {
    credentials: 'include',
    headers: { Authorization: `Bearer ${accessToken}` },
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json() as Promise<AuthUser>;
};
