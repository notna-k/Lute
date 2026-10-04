// The output contract: exit codes, the error shape, and where text goes. stdout carries
// only a command's result; progress, warnings and hints go to stderr.

import { styleText } from 'node:util';

/** Exit codes scripts can branch on. They are part of the CLI's contract. */
export const Exit = {
  ok: 0,
  /** With --wait or --follow: the run failed, timed out or was cancelled. */
  runFailed: 1,
  /** Bad usage or rejected input, or an action that is not possible. */
  usage: 2,
  /** No URL or key configured, or the key is invalid or revoked. */
  auth: 3,
  notFound: 4,
  /** Lute is unreachable or failed; safe to retry. */
  unavailable: 5,
  interrupted: 130,
} as const;

/** Error codes, the same set the API uses in error.code. */
const codeForExit: Record<number, string> = {
  [Exit.usage]: 'bad_request',
  [Exit.auth]: 'unauthorized',
  [Exit.notFound]: 'not_found',
  [Exit.unavailable]: 'unavailable',
};

/** CliError ends a command with a message on stderr and its exit code. */
export class CliError extends Error {
  readonly exit: number;
  readonly code: string;
  readonly fields: Record<string, string> | undefined;

  constructor(message: string, exit: number, opts: { code?: string; fields?: Record<string, string> } = {}) {
    super(message);
    this.exit = exit;
    this.code = opts.code ?? codeForExit[exit] ?? 'bad_request';
    this.fields = opts.fields;
  }
}

/** Ui writes for one invocation: human text, or JSON with --json. */
export class Ui {
  readonly json: boolean;

  constructor(json: boolean) {
    this.json = json;
  }

  /** out prints a line of the command's result. */
  out(line = ''): void {
    process.stdout.write(`${line}\n`);
  }

  /** info prints progress or a hint. */
  info(line = ''): void {
    if (!this.json) {
      process.stderr.write(`${line}\n`);
    }
  }

  /** warn prints a warning, even with --json, since stdout stays clean either way. */
  warn(line: string): void {
    process.stderr.write(`${color('yellow', line, process.stderr)}\n`);
  }

  /** printJson writes one JSON document, or one NDJSON record, to stdout. */
  printJson(value: unknown, pretty = true): void {
    process.stdout.write(`${JSON.stringify(value, null, pretty ? 2 : 0)}\n`);
  }

  /** error reports a failed command in the API's error shape with --json. */
  error(err: CliError): void {
    if (this.json) {
      const body: Record<string, unknown> = { code: err.code, message: err.message };
      if (err.fields) body.fields = err.fields;
      process.stderr.write(`${JSON.stringify({ error: body })}\n`);
      return;
    }
    process.stderr.write(`${color('red', `error: ${err.message}`, process.stderr)}\n`);
    for (const [field, problem] of Object.entries(err.fields ?? {})) {
      process.stderr.write(`  ${field}: ${problem}\n`);
    }
  }
}

type Style = Parameters<typeof styleText>[0];

/** color styles text only for a terminal, and never under NO_COLOR or TERM=dumb. */
export function color(style: Style, text: string, stream: NodeJS.WriteStream = process.stdout): string {
  if (!stream.isTTY || 'NO_COLOR' in process.env || process.env.TERM === 'dumb') {
    return text;
  }
  return styleText(style, text, { validateStream: false });
}

/** interactive is true when a person can answer prompts. */
export function interactive(): boolean {
  return Boolean(process.stdin.isTTY && process.stderr.isTTY);
}
