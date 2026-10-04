// What every command gets: its output, the global flags, and a way to reach Lute.

import { Client } from './client.ts';
import { checkTransport, resolveTarget } from './config.ts';
import { Ui } from './output.ts';
import { isPlainHttp } from './url.ts';

export interface GlobalFlags {
  json: boolean;
  url: string | undefined;
  allowInsecureHttp: boolean;
  debug: boolean;
}

export interface Ctx {
  ui: Ui;
  flags: GlobalFlags;
}

/** connect resolves the URL and key, and refuses plain HTTP unless it was allowed. */
export async function connect(ctx: Ctx): Promise<Client> {
  const target = await resolveTarget(ctx.flags.url, ctx.flags.allowInsecureHttp);
  checkTransport(target);
  if (isPlainHttp(target.url)) {
    ctx.ui.warn(`⚠ plain HTTP: ${target.url}`);
  }
  return new Client(target.url, target.key, ctx.flags.debug);
}

/** pollInterval is how often --wait and --follow ask for news. */
export function pollInterval(): number {
  const ms = Number(process.env.LUTE_POLL_INTERVAL_MS);
  return Number.isFinite(ms) && ms > 0 ? ms : 1000;
}

export const sleep = (ms: number) => new Promise((resolve) => setTimeout(resolve, ms));
