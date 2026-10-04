// Where the login lives: the URL in config.json, the key in the OS keychain (or, only when
// asked for, a 0600 file). Environment variables override both for one command.

import { mkdirSync, readFileSync, rmSync, writeFileSync, chmodSync } from 'node:fs';
import { homedir } from 'node:os';
import { join } from 'node:path';

import { CliError, Exit } from './output.ts';
import { isPlainHttp, normalizeUrl } from './url.ts';

export type Storage = 'keychain' | 'file';

/** Saved is config.json. It never holds the key. */
export interface Saved {
  url: string;
  storage: Storage;
  /** Consent, given at login, to send the key over plain HTTP to this URL. */
  allowInsecureHttp?: boolean;
}

export function configDir(): string {
  if (process.env.LUTE_CONFIG_DIR) return process.env.LUTE_CONFIG_DIR;
  if (process.platform === 'win32' && process.env.APPDATA) return join(process.env.APPDATA, 'lute');
  return join(process.env.XDG_CONFIG_HOME || join(homedir(), '.config'), 'lute');
}

const configFile = () => join(configDir(), 'config.json');
const credentialsFile = () => join(configDir(), 'credentials.json');

export function loadSaved(): Saved | undefined {
  try {
    const saved = JSON.parse(readFileSync(configFile(), 'utf8')) as Partial<Saved>;
    if (typeof saved.url !== 'string') return undefined;
    return { url: saved.url, storage: saved.storage === 'file' ? 'file' : 'keychain', allowInsecureHttp: saved.allowInsecureHttp };
  } catch {
    return undefined;
  }
}

export function writeSaved(saved: Saved): void {
  mkdirSync(configDir(), { recursive: true, mode: 0o700 });
  writeFileSync(configFile(), `${JSON.stringify(saved, null, 2)}\n`, { mode: 0o600 });
}

export function removeSaved(): void {
  rmSync(configFile(), { force: true });
}

/** KeyStore keeps one API key per Lute URL. */
export interface KeyStore {
  readonly kind: Storage;
  get(url: string): Promise<string | undefined>;
  set(url: string, key: string): Promise<void>;
  delete(url: string): Promise<void>;
}

const keychainService = 'lute-cli';

/** keychain is the OS credential store; it throws KeychainUnavailable when there is none. */
export async function keychain(): Promise<KeyStore> {
  if (process.env.LUTE_NO_KEYCHAIN) {
    throw new KeychainUnavailable('turned off by LUTE_NO_KEYCHAIN');
  }
  let mod: typeof import('@napi-rs/keyring');
  try {
    mod = await import('@napi-rs/keyring');
  } catch (err) {
    throw new KeychainUnavailable(`the keychain module did not load: ${(err as Error).message}`);
  }
  // Pinned to Secret Service on Linux: the kernel keyring it would fall back to forgets
  // the key at logout.
  const entry = (url: string) => new mod.AsyncEntry(keychainService, url, { linux: { store: 'secret-service' } });
  const wrap = async <T>(f: () => Promise<T>): Promise<T> => {
    try {
      return await f();
    } catch (err) {
      throw new KeychainUnavailable((err as Error).message);
    }
  };
  return {
    kind: 'keychain',
    get: (url) => wrap(() => entry(url).getPassword()),
    set: (url, key) => wrap(() => entry(url).setPassword(key)),
    delete: (url) => wrap(async () => void (await entry(url).deleteCredential())),
  };
}

export class KeychainUnavailable extends Error {}

/** fileStore keeps keys in a 0600 file, only with --insecure-storage. */
export function fileStore(): KeyStore {
  const read = (): Record<string, string> => {
    try {
      return JSON.parse(readFileSync(credentialsFile(), 'utf8')) as Record<string, string>;
    } catch {
      return {};
    }
  };
  const write = (all: Record<string, string>) => {
    if (Object.keys(all).length === 0) {
      rmSync(credentialsFile(), { force: true });
      return;
    }
    mkdirSync(configDir(), { recursive: true, mode: 0o700 });
    writeFileSync(credentialsFile(), `${JSON.stringify(all, null, 2)}\n`, { mode: 0o600 });
    chmodSync(credentialsFile(), 0o600);
  };
  return {
    kind: 'file',
    get: async (url) => read()[url],
    set: async (url, key) => write({ ...read(), [url]: key }),
    delete: async (url) => {
      const all = read();
      delete all[url];
      write(all);
    },
  };
}

export async function storeFor(storage: Storage): Promise<KeyStore> {
  return storage === 'file' ? fileStore() : keychain();
}

/** Target is the server one command talks to. */
export interface Target {
  url: string;
  key: string | undefined;
  /** The key may be sent over plain HTTP: by flag, or by consent saved at login. */
  insecureOk: boolean;
}

/**
 * resolveTarget picks the URL from --url, then LUTE_URL, then the saved login, and the key
 * from LUTE_API_KEY, then the saved login. A saved key is only ever sent to the URL it was
 * saved for.
 */
export async function resolveTarget(flagUrl: string | undefined, allowInsecureHttp: boolean): Promise<Target> {
  const saved = loadSaved();
  const given = flagUrl ?? process.env.LUTE_URL;
  const url = given ? normalizeUrl(given) : saved?.url;
  if (!url) {
    throw new CliError('not logged in: run "lute login", or set LUTE_URL and LUTE_API_KEY', Exit.auth);
  }
  const isSaved = saved?.url === url;
  let key = process.env.LUTE_API_KEY || undefined;
  if (!key && isSaved && saved) {
    try {
      key = await (await storeFor(saved.storage)).get(url);
    } catch (err) {
      if (!(err instanceof KeychainUnavailable)) throw err;
      throw new CliError(`the saved API key could not be read from the keychain: ${err.message}`, Exit.auth);
    }
  }
  return { url, key, insecureOk: allowInsecureHttp || (isSaved && saved?.allowInsecureHttp === true) };
}

/** checkTransport refuses to send a key over plain HTTP without consent. */
export function checkTransport(target: Pick<Target, 'url' | 'insecureOk'>): void {
  if (isPlainHttp(target.url) && !target.insecureOk) {
    throw new CliError(
      `${target.url} uses plain HTTP, not HTTPS. Your API key would cross the network unencrypted, ` +
        'so nothing was sent. Put Lute behind HTTPS, or pass --allow-insecure-http to send it anyway.',
      Exit.usage,
    );
  }
}
