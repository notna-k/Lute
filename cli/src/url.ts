// What the CLI accepts as a Lute URL, and when it counts as plain HTTP.

import { CliError, Exit } from './output.ts';

/**
 * normalizeUrl turns what a person types into the panel's base URL: https:// is assumed
 * when no scheme is given, and a trailing slash or path is dropped.
 */
export function normalizeUrl(input: string): string {
  let raw = input.trim();
  if (raw === '') {
    throw new CliError('the Lute URL is empty', Exit.usage);
  }
  if (!/^[a-z][a-z0-9+.-]*:\/\//i.test(raw)) {
    raw = `https://${raw}`;
  }
  let url: URL;
  try {
    url = new URL(raw);
  } catch {
    throw new CliError(`"${input}" is not a URL`, Exit.usage);
  }
  if (url.protocol !== 'https:' && url.protocol !== 'http:') {
    throw new CliError(`"${input}" must be an http:// or https:// URL`, Exit.usage);
  }
  if (url.username || url.password) {
    throw new CliError('the Lute URL must not carry a user name or password', Exit.usage);
  }
  return url.origin;
}

/** The key never leaves the machine for these hosts, so plain HTTP is fine. */
export function isLoopback(url: string): boolean {
  const host = new URL(url).hostname;
  return host === 'localhost' || host === '[::1]' || /^127(\.\d{1,3}){3}$/.test(host);
}

/** isPlainHttp is true when the key would cross a network unencrypted. */
export function isPlainHttp(url: string): boolean {
  return new URL(url).protocol === 'http:' && !isLoopback(url);
}
