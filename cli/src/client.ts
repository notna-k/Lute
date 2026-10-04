// A thin fetch wrapper for /api/public/v1. It maps HTTP failures to exit codes and never
// prints the key, not even with --debug.

import type { components } from './api.ts';
import { CliError, Exit } from './output.ts';

export type Run = components['schemas']['Run'];
export type RunList = components['schemas']['RunList'];
export type RunStatus = components['schemas']['RunStatus'];
export type JobSummary = components['schemas']['JobSummary'];
export type JobDetail = components['schemas']['JobDetail'];
export type Parameter = components['schemas']['Parameter'];
export type LogPage = components['schemas']['LogPage'];
export type WhoAmI = components['schemas']['WhoAmI'];
export type Version = components['schemas']['Version'];

/** API_LEVEL is the public API level this CLI was built against. */
export const API_LEVEL = 1;

export interface RequestOptions {
  body?: unknown;
  query?: Record<string, string | number | undefined>;
  headers?: Record<string, string>;
  /** auth: false sends no key, for /version. */
  auth?: boolean;
}

export class Client {
  readonly url: string;
  private readonly key: string | undefined;
  private readonly debug: boolean;

  constructor(url: string, key: string | undefined, debug = false) {
    this.url = url;
    this.key = key;
    this.debug = debug;
  }

  async request<T>(method: string, path: string, opts: RequestOptions = {}): Promise<{ status: number; data: T }> {
    const target = new URL(`${this.url}/api/public/v1${path}`);
    for (const [k, v] of Object.entries(opts.query ?? {})) {
      if (v !== undefined && v !== '') target.searchParams.set(k, String(v));
    }
    const headers: Record<string, string> = { Accept: 'application/json', ...opts.headers };
    if (opts.auth !== false) {
      if (!this.key) {
        throw new CliError('no API key: run "lute login", or set LUTE_API_KEY', Exit.auth);
      }
      headers.Authorization = `Bearer ${this.key}`;
    }
    if (opts.body !== undefined) headers['Content-Type'] = 'application/json';

    const started = Date.now();
    this.trace(`→ ${method} ${target}`);
    if (headers.Authorization) this.trace(`  Authorization: Bearer ${redact(this.key ?? '')}`);

    let res: Response;
    try {
      res = await fetch(target, {
        method,
        headers,
        body: opts.body === undefined ? undefined : JSON.stringify(opts.body),
      });
    } catch (err) {
      const cause = (err as { cause?: { message?: string } }).cause?.message ?? (err as Error).message;
      throw new CliError(`could not reach Lute at ${this.url}: ${cause}`, Exit.unavailable);
    }
    const text = await res.text();
    this.trace(`← ${res.status} in ${Date.now() - started}ms`);

    if (!res.ok) throw httpError(res.status, text);
    return { status: res.status, data: (text ? JSON.parse(text) : undefined) as T };
  }

  async get<T>(path: string, opts: RequestOptions = {}): Promise<T> {
    return (await this.request<T>('GET', path, opts)).data;
  }

  private trace(line: string): void {
    if (this.debug) process.stderr.write(`${line}\n`);
  }
}

/** redact keeps the public prefix, which the panel shows anyway, and hides the rest. */
export function redact(key: string): string {
  return key.startsWith('lute_sk_') ? `${key.slice(0, 16)}…(redacted)` : '(redacted)';
}

/** exitForStatus maps an HTTP status to the CLI's exit code. */
export function exitForStatus(status: number): number {
  if (status === 401 || status === 403) return Exit.auth;
  if (status === 404) return Exit.notFound;
  if (status >= 500) return Exit.unavailable;
  return Exit.usage;
}

function httpError(status: number, text: string): CliError {
  let detail: { code?: string; message?: string; fields?: Record<string, string> } = {};
  try {
    detail = (JSON.parse(text) as { error?: typeof detail }).error ?? {};
  } catch {
    // Not the API's error shape, e.g. a proxy's HTML page.
  }
  let message = detail.message || `HTTP ${status}`;
  if (status === 401) message = `the API key was refused (${message}); it is wrong or revoked`;
  return new CliError(message, exitForStatus(status), { code: detail.code, fields: detail.fields });
}
