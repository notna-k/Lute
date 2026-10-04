// lute logs, and the wait/follow loops lute run shares.

import type { Client, LogPage, Run } from '../client.ts';
import { connect, pollInterval, sleep, type Ctx } from '../context.ts';
import { cleanRef, isFinal, passed, summary } from '../format.ts';
import { CliError, Exit, type Ui } from '../output.ts';

const pageSize = 500;

export interface LogsFlags {
  follow: boolean;
  tail: number | undefined;
}

export async function logs(ctx: Ctx, ref: string, flags: LogsFlags): Promise<number> {
  const client = await connect(ctx);
  const run = await client.get<Run>(`/runs/${encodeURIComponent(cleanRef(ref))}`);
  if (flags.follow) {
    const final = await follow(client, ctx.ui, run.id);
    return finish(ctx.ui, final, true);
  }
  const lines = flags.tail === undefined ? await readAll(client, run.id) : await readTail(client, run.id, flags.tail);
  if (ctx.ui.json) ctx.ui.printJson({ id: run.id, lines });
  else for (const line of lines) ctx.ui.out(line);
  return Exit.ok;
}

async function readAll(client: Client, id: string): Promise<string[]> {
  const out: string[] = [];
  let cursor: string | undefined;
  for (;;) {
    const page = await client.get<LogPage>(`/runs/${id}/logs`, { query: { direction: 'head', limit: pageSize, cursor } });
    out.push(...page.lines);
    if (!page.has_more || !page.next_cursor) return out;
    cursor = page.next_cursor;
  }
}

async function readTail(client: Client, id: string, n: number): Promise<string[]> {
  let out: string[] = [];
  let cursor: string | undefined;
  while (out.length < n) {
    const limit = Math.min(pageSize, n - out.length);
    const page = await client.get<LogPage>(`/runs/${id}/logs`, { query: { direction: 'tail', limit, cursor } });
    out = [...page.lines, ...out];
    if (!page.has_more || !page.next_cursor) break;
    cursor = page.next_cursor;
  }
  return out.slice(-n);
}

/** wait polls the run until it ends. */
export async function wait(client: Client, id: string): Promise<Run> {
  for (;;) {
    const run = await client.get<Run>(`/runs/${id}`);
    if (isFinal(run.status)) return run;
    await sleep(pollInterval());
  }
}

/**
 * follow prints the log as it grows until the run ends. The run's status is read before
 * each log page, so the last page after it ended holds every line.
 */
export async function follow(client: Client, ui: Ui, id: string): Promise<Run> {
  let cursor: string | undefined;
  let warned = false;
  for (;;) {
    const run = await client.get<Run>(`/runs/${id}`);
    let more = false;
    try {
      const page = await client.get<LogPage>(`/runs/${id}/logs`, { query: { direction: 'head', limit: pageSize, cursor } });
      for (const line of page.lines) {
        if (ui.json) ui.printJson({ line }, false);
        else ui.out(line);
      }
      if (page.next_cursor) cursor = page.next_cursor;
      more = page.has_more;
    } catch (err) {
      // No log before a worker takes the run (404), or while its worker is away (503).
      if (!(err instanceof CliError) || (err.exit !== Exit.notFound && err.exit !== Exit.unavailable)) throw err;
      if (isFinal(run.status) && !warned) {
        ui.warn(`the log is not available: ${err.message}`);
      }
      warned = true;
    }
    if (more) continue;
    if (isFinal(run.status)) return run;
    await sleep(pollInterval());
  }
}

/**
 * finish prints how a waited-for run ended; a failed run exits 1. With --json a followed
 * run ends its NDJSON with {"run": ...}, and a waited-for one is the run itself.
 */
export function finish(ui: Ui, run: Run, followed: boolean): number {
  if (!ui.json) ui.out(summary(run));
  else if (followed) ui.printJson({ run }, false);
  else ui.printJson(run);
  return passed(run) ? Exit.ok : Exit.runFailed;
}
