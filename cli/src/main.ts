#!/usr/bin/env node
// The lute command: parse the arguments, run one command, exit with its code.

import { parseArgs, type ParseArgsConfig } from 'node:util';

import { login, logout } from './commands/login.ts';
import { listJobs, showJob } from './commands/jobs.ts';
import { logs } from './commands/logs.ts';
import { cancel, listRuns, retry, run, showRun } from './commands/runs.ts';
import { cliVersion, version, whoami } from './commands/whoami.ts';
import type { Ctx } from './context.ts';
import { CliError, Exit, Ui } from './output.ts';

type Options = NonNullable<ParseArgsConfig['options']>;
type Values = Record<string, string | boolean | string[] | undefined>;

interface Command {
  usage: string;
  summary: string;
  options?: Options;
  /** positionals is how many arguments the command takes. */
  positionals?: number;
  run: (ctx: Ctx, args: string[], values: Values) => Promise<number>;
}

const globalOptions: Options = {
  json: { type: 'boolean' },
  url: { type: 'string' },
  'allow-insecure-http': { type: 'boolean' },
  debug: { type: 'boolean' },
  help: { type: 'boolean', short: 'h' },
};

const num = (v: unknown, flag: string): number | undefined => {
  if (v === undefined) return undefined;
  const n = Number(v);
  if (!Number.isInteger(n) || n < 0) throw new CliError(`--${flag} must be a whole number`, Exit.usage);
  return n;
};

const commands: Record<string, Command> = {
  login: {
    usage: 'lute login [--url URL] [--api-key-stdin] [--allow-insecure-http] [--insecure-storage]',
    summary: 'Connect to Lute with its URL and an API key; the key goes to the system keychain',
    options: { 'api-key-stdin': { type: 'boolean' }, 'insecure-storage': { type: 'boolean' } },
    run: (ctx, _, v) => login(ctx, { apiKeyStdin: v['api-key-stdin'] === true, insecureStorage: v['insecure-storage'] === true }),
  },
  logout: {
    usage: 'lute logout',
    summary: 'Remove the saved URL and key from this machine',
    run: (ctx) => logout(ctx),
  },
  whoami: {
    usage: 'lute whoami',
    summary: 'Show the URL, the key and who it acts as (never the key itself)',
    run: (ctx) => whoami(ctx),
  },
  jobs: {
    usage: 'lute jobs\n  lute jobs show <slug>',
    summary: 'List jobs, or show one with its parameters',
    positionals: 2,
    run: (ctx, args) => {
      if (args.length === 0) return listJobs(ctx);
      if (args[0] === 'show' && args[1]) return showJob(ctx, args[1]);
      throw new CliError('usage: lute jobs | lute jobs show <slug>', Exit.usage);
    },
  },
  run: {
    usage: 'lute run <slug> [-p name=value]... [--wait | --follow] [--idempotency-key KEY]',
    summary: 'Start a run of a job; -p repeats, a,b sets a list, name=@file reads a file',
    positionals: 1,
    options: {
      param: { type: 'string', short: 'p', multiple: true },
      wait: { type: 'boolean' },
      follow: { type: 'boolean' },
      'idempotency-key': { type: 'string' },
    },
    run: (ctx, args, v) => {
      const slug = required(args[0], 'lute run <slug>');
      return run(ctx, slug, {
        params: (v.param as string[] | undefined) ?? [],
        wait: v.wait === true,
        follow: v.follow === true,
        idempotencyKey: v['idempotency-key'] as string | undefined,
      });
    },
  },
  runs: {
    usage: 'lute runs [--job SLUG] [--status STATUS] [--limit N] [--offset N]\n  lute runs show <id>',
    summary: 'List recent runs, newest first, or show one',
    positionals: 2,
    options: {
      job: { type: 'string' },
      status: { type: 'string' },
      limit: { type: 'string' },
      offset: { type: 'string' },
    },
    run: (ctx, args, v) => {
      if (args[0] === 'show') return showRun(ctx, required(args[1], 'lute runs show <id>'));
      if (args.length > 0) throw new CliError('usage: lute runs | lute runs show <id>', Exit.usage);
      return listRuns(ctx, {
        job: v.job as string | undefined,
        status: v.status as string | undefined,
        limit: num(v.limit, 'limit'),
        offset: num(v.offset, 'offset'),
      });
    },
  },
  logs: {
    usage: 'lute logs <id> [-f] [--tail N]',
    summary: 'Print a run\'s log; -f follows it until the run ends (Ctrl-C never cancels the run)',
    positionals: 1,
    options: { follow: { type: 'boolean', short: 'f' }, tail: { type: 'string' } },
    run: (ctx, args, v) => logs(ctx, required(args[0], 'lute logs <id>'), { follow: v.follow === true, tail: num(v.tail, 'tail') }),
  },
  cancel: {
    usage: 'lute cancel <id>',
    summary: 'Cancel a run no worker has taken yet',
    positionals: 1,
    run: (ctx, args) => cancel(ctx, required(args[0], 'lute cancel <id>')),
  },
  retry: {
    usage: 'lute retry <id>',
    summary: 'Queue a run again with the same parameters',
    positionals: 1,
    run: (ctx, args) => retry(ctx, required(args[0], 'lute retry <id>')),
  },
  version: {
    usage: 'lute version',
    summary: 'Show the lute and server versions',
    run: (ctx) => version(ctx),
  },
};

function required(value: string | undefined, usage: string): string {
  if (!value) throw new CliError(`usage: ${usage}`, Exit.usage);
  return value;
}

function help(): string {
  const width = Math.max(...Object.keys(commands).map((c) => c.length));
  return [
    `lute ${cliVersion} — run and follow Lute jobs from a terminal, a script or CI.`,
    '',
    'Commands:',
    ...Object.entries(commands).map(([name, c]) => `  ${name.padEnd(width)}  ${c.summary}`),
    '',
    'Global flags:',
    '  --json                 machine output: one JSON document on stdout, errors as JSON on stderr',
    '  --url URL              talk to this Lute for one command (or LUTE_URL)',
    '  --allow-insecure-http  send the key to a plain http:// URL anyway',
    '  --debug                print each request to stderr, with the key redacted',
    '',
    'Without a login, LUTE_URL and LUTE_API_KEY are used. Run IDs may be the short form, #a1b2c3d4.',
    'Exit codes: 0 ok · 1 run failed · 2 usage or rejected input · 3 auth · 4 not found · 5 unreachable · 130 interrupted',
    'Run "lute <command> --help" for its flags.',
  ].join('\n');
}

/** commandName finds the first argument that is not a flag or a global flag's value. */
function commandName(argv: string[]): string | undefined {
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i] as string;
    if (a === '--url') {
      i++;
      continue;
    }
    if (!a.startsWith('-')) return a;
  }
  return undefined;
}

export async function main(argv: string[]): Promise<number> {
  let ui = new Ui(argv.includes('--json'));
  try {
    const name = commandName(argv);
    if (name === undefined || name === 'help') {
      if (argv.includes('--version')) {
        ui.out(cliVersion);
        return Exit.ok;
      }
      ui.out(help());
      return Exit.ok;
    }
    const cmd = commands[name];
    if (!cmd) throw new CliError(`unknown command "${name}"\n\n${help()}`, Exit.usage);

    const rest = [...argv];
    rest.splice(rest.indexOf(name), 1);
    let parsed;
    try {
      parsed = parseArgs({ args: rest, options: { ...globalOptions, ...cmd.options }, allowPositionals: true, strict: true });
    } catch (err) {
      throw new CliError(`${(err as Error).message}\nusage: ${cmd.usage}`, Exit.usage);
    }
    const v = parsed.values as Values;
    if (v.help) {
      ui.out(`${cmd.summary}.\n\nusage: ${cmd.usage}`);
      return Exit.ok;
    }
    if (parsed.positionals.length > (cmd.positionals ?? 0)) {
      throw new CliError(`too many arguments\nusage: ${cmd.usage}`, Exit.usage);
    }
    ui = new Ui(v.json === true);
    const ctx: Ctx = {
      ui,
      flags: {
        json: v.json === true,
        url: v.url as string | undefined,
        allowInsecureHttp: v['allow-insecure-http'] === true,
        debug: v.debug === true,
      },
    };
    return await cmd.run(ctx, parsed.positionals, v);
  } catch (err) {
    const e = err instanceof CliError ? err : new CliError((err as Error).message ?? String(err), Exit.unavailable, { code: 'internal' });
    ui.error(e);
    return e.exit;
  }
}

// Ctrl-C ends the command, never the run it may be following.
process.on('SIGINT', () => {
  process.stderr.write('\n');
  process.exit(Exit.interrupted);
});

process.exitCode = await main(process.argv.slice(2));
