// Test helpers: the real lute process against a small stand-in for Lute's public API.

import { spawn } from 'node:child_process';
import { mkdtempSync } from 'node:fs';
import { createServer, type IncomingMessage, type Server, type ServerResponse } from 'node:http';
import type { AddressInfo } from 'node:net';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

export const KEY = 'lute_sk_abcdefgh12345678secretpart';

const main = join(import.meta.dirname, '..', 'src', 'main.ts');

export interface Result {
  code: number | null;
  stdout: string;
  stderr: string;
}

export interface CliOptions {
  env?: Record<string, string | undefined>;
  stdin?: string;
  configDir?: string;
}

/** lute runs the CLI as a child process, never touching the real keychain or config. */
export function lute(args: string[], opts: CliOptions = {}): Promise<Result> {
  const env: Record<string, string> = {
    PATH: process.env.PATH ?? '',
    HOME: process.env.HOME ?? '',
    LUTE_CONFIG_DIR: opts.configDir ?? freshConfigDir(),
    LUTE_NO_KEYCHAIN: '1',
    LUTE_POLL_INTERVAL_MS: '10',
    NO_COLOR: '1',
  };
  for (const [k, v] of Object.entries(opts.env ?? {})) {
    if (v === undefined) delete env[k];
    else env[k] = v;
  }
  return new Promise((resolve, reject) => {
    const child = spawn(process.execPath, [main, ...args], { env, stdio: ['pipe', 'pipe', 'pipe'] });
    let stdout = '';
    let stderr = '';
    child.stdout.on('data', (d) => (stdout += d));
    child.stderr.on('data', (d) => (stderr += d));
    child.on('error', reject);
    child.on('close', (code) => resolve({ code, stdout, stderr }));
    child.stdin.end(opts.stdin ?? '');
  });
}

export function freshConfigDir(): string {
  return mkdtempSync(join(tmpdir(), 'lute-cli-test-'));
}

export interface Seen {
  method: string;
  path: string;
  query: URLSearchParams;
  headers: IncomingMessage['headers'];
  body: unknown;
}

type Handler = (req: Seen) => { status?: number; body?: unknown } | undefined;

/** FakeLute answers like core's public API; each test scripts the routes it needs. */
export class FakeLute {
  readonly seen: Seen[] = [];
  private readonly server: Server;
  private readonly routes: [string, string | RegExp, Handler][] = [];
  url = '';

  constructor() {
    this.server = createServer((req, res) => void this.handle(req, res));
    this.on('GET', '/version', () => ({ body: { version: '0.9.0', api_level: 1 } }));
  }

  /** on adds a route; later routes win, so a test can override a default. */
  on(method: string, path: string | RegExp, handler: Handler): this {
    this.routes.unshift([method, path, handler]);
    return this;
  }

  async start(host = '127.0.0.1'): Promise<this> {
    await new Promise<void>((resolve) => this.server.listen(0, host, resolve));
    const { port } = this.server.address() as AddressInfo;
    this.url = `http://${host.includes(':') ? `[${host}]` : host}:${port}`;
    return this;
  }

  stop(): Promise<void> {
    return new Promise((resolve) => this.server.close(() => resolve()));
  }

  /** requests are the calls that reached the API, /version aside. */
  requests(): Seen[] {
    return this.seen.filter((s) => s.path !== '/version');
  }

  private async handle(req: IncomingMessage, res: ServerResponse): Promise<void> {
    let raw = '';
    for await (const chunk of req) raw += chunk;
    const url = new URL(req.url ?? '/', 'http://x');
    const path = url.pathname.replace(/^\/api\/public\/v1/, '');
    const seen: Seen = {
      method: req.method ?? 'GET',
      path,
      query: url.searchParams,
      headers: req.headers,
      body: raw ? JSON.parse(raw) : undefined,
    };
    this.seen.push(seen);

    if (path !== '/version' && req.headers.authorization !== `Bearer ${KEY}`) {
      return send(res, 401, { error: { code: 'unauthorized', message: 'invalid API key' } });
    }
    for (const [method, pattern, handler] of this.routes) {
      const match = typeof pattern === 'string' ? pattern === path : pattern.test(path);
      if (method === seen.method && match) {
        const out = handler(seen);
        if (out) return send(res, out.status ?? 200, out.body);
      }
    }
    send(res, 404, { error: { code: 'not_found', message: `no route ${seen.method} ${path}` } });
  }
}

function send(res: ServerResponse, status: number, body: unknown): void {
  res.writeHead(status, { 'Content-Type': 'application/json' });
  res.end(body === undefined ? '' : JSON.stringify(body));
}

export const RUN_ID = '66fb1c2d3e4f5a6b7c8d9e0f';

export function aRun(fields: Record<string, unknown> = {}): Record<string, unknown> {
  return {
    id: RUN_ID,
    job: 'web-release',
    queue: 'deploy',
    type: 'container',
    status: 'pending',
    attempts: 0,
    max_retries: 3,
    timeout_sec: 300,
    enqueued_at: new Date().toISOString(),
    ...fields,
  };
}

export const webRelease = {
  slug: 'web-release',
  name: 'Web release',
  queue: 'deploy',
  runtime: 'node:25-alpine',
  command: './ship.sh',
  parameters: [
    { name: 'environment', type: 'select', env_var: 'ENVIRONMENT', required: true, options: [{ value: 'staging' }, { value: 'prod' }] },
    { name: 'regions', type: 'multiselect', env_var: 'REGIONS', required: false, options: [{ value: 'eu' }, { value: 'us' }] },
    { name: 'notes', type: 'string', env_var: 'NOTES', required: false },
    { name: 'token', type: 'secret', env_var: 'TOKEN', required: false },
  ],
};

/** jsonLines parses NDJSON. */
export function jsonLines(text: string): unknown[] {
  return text
    .split('\n')
    .filter((l) => l.trim() !== '')
    .map((l) => JSON.parse(l) as unknown);
}
