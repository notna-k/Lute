// lute login / lute logout.

import { text } from 'node:stream/consumers';

import { Client, type Version, type WhoAmI } from '../client.ts';
import {
  fileStore,
  keychain,
  KeychainUnavailable,
  loadSaved,
  removeSaved,
  storeFor,
  writeSaved,
  type KeyStore,
} from '../config.ts';
import type { Ctx } from '../context.ts';
import { CliError, color, Exit, interactive } from '../output.ts';
import { ask, askHidden, confirm } from '../prompt.ts';
import { isPlainHttp, normalizeUrl } from '../url.ts';

export interface LoginFlags {
  apiKeyStdin: boolean;
  insecureStorage: boolean;
}

const httpWarning = [
  'WARNING: this Lute URL uses plain HTTP, not HTTPS',
  '',
  'Your API key and every job parameter would cross the',
  'network unencrypted. Anyone on the path could read the key',
  'and run jobs with it.',
  '',
  'Put Lute behind HTTPS, or continue only on a network you',
  'trust completely.',
];

function box(lines: string[]): string {
  const width = Math.max(...lines.map((l) => l.length)) + 4;
  const bar = '─'.repeat(width);
  return [`┌${bar}┐`, ...lines.map((l) => `│  ${l.padEnd(width - 2)}│`), `└${bar}┘`].join('\n');
}

export async function login(ctx: Ctx, flags: LoginFlags): Promise<number> {
  const { ui } = ctx;
  const tty = interactive() && !flags.apiKeyStdin;

  // 1. The URL, and consent before anything is sent over plain HTTP.
  let rawUrl = ctx.flags.url ?? process.env.LUTE_URL;
  if (!rawUrl) {
    if (!tty) throw new CliError('no Lute URL: pass --url', Exit.usage);
    rawUrl = await ask('Lute URL');
  }
  const url = normalizeUrl(rawUrl);
  const plainHttp = isPlainHttp(url);
  if (plainHttp && !ctx.flags.allowInsecureHttp) {
    if (!tty) {
      throw new CliError(
        `${url} uses plain HTTP, not HTTPS. Your API key would cross the network unencrypted, so nothing ` +
          'was sent. Put Lute behind HTTPS, or pass --allow-insecure-http to send it anyway.',
        Exit.usage,
      );
    }
    process.stderr.write(`${color('yellow', box(httpWarning), process.stderr)}\n`);
    if (!(await confirm('Continue over plain HTTP?'))) {
      throw new CliError('login stopped; nothing was sent', Exit.usage);
    }
  }

  // 2. The key: hidden on a terminal, piped otherwise; never a flag, which would land in
  //    shell history and ps.
  let key: string;
  if (flags.apiKeyStdin) {
    key = (await text(process.stdin)).trim();
  } else if (tty) {
    key = await askHidden('API key');
  } else {
    throw new CliError('no API key: pipe it in with --api-key-stdin', Exit.usage);
  }
  if (!key) throw new CliError('the API key is empty', Exit.usage);
  if (!key.startsWith('lute_sk_')) {
    throw new CliError('that is not a Lute API key; keys start with lute_sk_', Exit.usage);
  }

  // 3. Check it before saving anything.
  const client = new Client(url, key, ctx.flags.debug);
  const version = await client.get<Version>('/version', { auth: false });
  const who = await client.get<WhoAmI>('/whoami');

  // 4. Save: the URL in config.json, the key in the keychain unless a file was asked for.
  const store = await pickStore(flags.insecureStorage);
  const previous = loadSaved();
  if (previous) await forgetKey(previous.url, previous.storage);
  await store.set(url, key);
  writeSaved({ url, storage: store.kind, ...(plainHttp ? { allowInsecureHttp: true } : {}) });

  if (ui.json) {
    ui.printJson({ url, version: version.version, api_level: version.api_level, ...who, storage: store.kind });
    return Exit.ok;
  }
  const as = who.user ? who.user.email : `service key "${who.key.name}"`;
  ui.out(`${color('green', '✓')} Connected to Lute ${version.version} as ${as}`);
  const where = store.kind === 'keychain' ? 'the system keychain' : 'a 0600 file (--insecure-storage)';
  ui.out(color('dim', `  ${who.key.scope} key "${who.key.name}" · saved to ${where}`));
  if (plainHttp) ui.warn(`⚠ plain HTTP: ${url}`);
  return Exit.ok;
}

async function pickStore(insecureStorage: boolean): Promise<KeyStore> {
  if (insecureStorage) return fileStore();
  try {
    const store = await keychain();
    // Probe now, so a missing keychain stops the login rather than the next command.
    await store.get('probe://lute');
    return store;
  } catch (err) {
    if (!(err instanceof KeychainUnavailable)) throw err;
    throw new CliError(
      `no system keychain is available to keep the API key (${err.message}). On a desktop, unlock or ` +
        'install a keychain (Secret Service on Linux). On a headless machine, prefer LUTE_URL and ' +
        'LUTE_API_KEY, or pass --insecure-storage to keep the key in a 0600 file.',
      Exit.usage,
    );
  }
}

async function forgetKey(url: string, storage: 'keychain' | 'file'): Promise<void> {
  try {
    await (await storeFor(storage)).delete(url);
  } catch (err) {
    if (!(err instanceof KeychainUnavailable)) throw err;
  }
}

export async function logout(ctx: Ctx): Promise<number> {
  const { ui } = ctx;
  const saved = loadSaved();
  if (!saved) {
    if (ui.json) ui.printJson({ logged_out: false });
    else ui.out('Not logged in.');
    return Exit.ok;
  }
  await forgetKey(saved.url, saved.storage);
  removeSaved();
  if (ui.json) {
    ui.printJson({ url: saved.url, logged_out: true });
    return Exit.ok;
  }
  ui.out(`${color('green', '✓')} Logged out of ${saved.url}; the key is gone from this machine.`);
  ui.info(color('dim', `  It still works until you revoke it in the panel: ${saved.url}/settings`));
  return Exit.ok;
}
