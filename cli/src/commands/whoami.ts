// lute whoami / lute version.

import { createRequire } from 'node:module';

import { API_LEVEL, Client, type Version, type WhoAmI } from '../client.ts';
import { checkTransport, resolveTarget } from '../config.ts';
import { connect, type Ctx } from '../context.ts';
import { CliError, Exit } from '../output.ts';

export const cliVersion: string = (createRequire(import.meta.url)('../../package.json') as { version: string }).version;

export async function whoami(ctx: Ctx): Promise<number> {
  const client = await connect(ctx);
  const who = await client.get<WhoAmI>('/whoami');
  if (ctx.ui.json) {
    ctx.ui.printJson({ url: client.url, ...who });
    return Exit.ok;
  }
  const { ui } = ctx;
  ui.out(`URL      ${client.url}`);
  ui.out(`Key      "${who.key.name}" (${who.key.scope} key, ${who.key.prefix}…)`);
  ui.out(`Acts as  ${who.user ? who.user.email : 'itself: a service key belongs to this Lute, not to a person'}`);
  return Exit.ok;
}

/** version needs no key: it reports the client, and the server when one is configured. */
export async function version(ctx: Ctx): Promise<number> {
  const { ui } = ctx;
  let server: Version | undefined;
  let url: string | undefined;
  try {
    const target = await resolveTarget(ctx.flags.url, ctx.flags.allowInsecureHttp);
    checkTransport(target);
    url = target.url;
    server = await new Client(target.url, undefined, ctx.flags.debug).get<Version>('/version', { auth: false });
  } catch (err) {
    // No login yet is fine for a version check; an unreachable server is worth saying.
    if (!(err instanceof CliError) || err.exit !== Exit.auth) {
      if (url) ui.warn(`could not read the server version: ${(err as Error).message}`);
    }
  }
  const mismatch = server !== undefined && server.api_level !== API_LEVEL;

  if (ui.json) {
    ui.printJson({ client: cliVersion, api_level: API_LEVEL, url: url ?? null, server: server ?? null });
  } else {
    ui.out(`lute    ${cliVersion} (API level ${API_LEVEL})`);
    ui.out(server ? `server  ${server.version} (API level ${server.api_level}) at ${url}` : 'server  not connected');
  }
  if (mismatch) {
    ui.warn(
      `⚠ this lute speaks API level ${API_LEVEL} and the server level ${server?.api_level}; ` +
        `update ${server && server.api_level > API_LEVEL ? 'lute (npm i -g @notna-k/lute-cli)' : 'the server'}`,
    );
  }
  return Exit.ok;
}
