// lute run, lute runs, lute runs show, lute cancel, lute retry.

import type { JobDetail, Parameter, Run, RunList } from '../client.ts';
import { connect, type Ctx } from '../context.ts';
import { ago, cleanRef, duration, runLink, shortId, statusText, table } from '../format.ts';
import { CliError, color, Exit, interactive } from '../output.ts';
import { missingRequired, parseParamFlags, toParams } from '../params.ts';
import { ask } from '../prompt.ts';
import { finish, follow, wait } from './logs.ts';

export interface RunFlags {
  params: string[];
  wait: boolean;
  follow: boolean;
  idempotencyKey: string | undefined;
}

export async function run(ctx: Ctx, slug: string, flags: RunFlags): Promise<number> {
  const { ui } = ctx;
  const client = await connect(ctx);
  const raw = parseParamFlags(flags.params);
  const job = await client.get<JobDetail>(`/jobs/${encodeURIComponent(slug)}`);
  const params = toParams(raw, job.parameters);

  // A person is asked for what is missing; a script gets core's validation error instead.
  if (interactive() && !ui.json) {
    for (const p of missingRequired(params, job.parameters)) {
      params[p.name] = await promptFor(p);
    }
  }

  const { status, data: started } = await client.request<Run>('POST', `/jobs/${encodeURIComponent(slug)}/runs`, {
    body: { params },
    headers: flags.idempotencyKey ? { 'Idempotency-Key': flags.idempotencyKey } : undefined,
  });
  const queued = `${color('green', '✓')} ${status === 200 ? 'Found earlier run' : 'Queued run'} ${color('bold', `#${shortId(started.id)}`)} on queue ${started.queue}`;
  const link = runLink(client.url, started);

  if (!flags.wait && !flags.follow) {
    if (ui.json) {
      ui.printJson(started);
    } else {
      ui.out(queued);
      if (link) ui.out(color('dim', `  ${link}`));
    }
    return Exit.ok;
  }
  // The log is the result now, so the queued line is progress.
  ui.info(queued);
  if (link) ui.info(color('dim', `  ${link}`));
  if (flags.follow) {
    return finish(ui, await follow(client, ui, started.id), true);
  }
  return finish(ui, await wait(client, started.id), false);
}

async function promptFor(p: Parameter): Promise<unknown> {
  const what = p.label && p.label !== p.name ? `${p.label} (${p.name})` : p.name;
  if (p.options?.length) {
    p.options.forEach((o, i) => process.stderr.write(`  ${i + 1}) ${o.label && o.label !== o.value ? `${o.value} — ${o.label}` : o.value}\n`));
    const pick = (answer: string) => {
      const i = Number(answer);
      return Number.isInteger(i) && i >= 1 && i <= (p.options?.length ?? 0) ? (p.options?.[i - 1]?.value ?? answer) : answer;
    };
    if (p.type === 'multiselect') {
      const answer = await ask(what, '(numbers or values, comma-separated)');
      return answer
        .split(',')
        .map((a) => pick(a.trim()))
        .filter(Boolean);
    }
    return pick(await ask(what, '(number or value)'));
  }
  return ask(what, p.type === 'bool' ? '(true/false)' : p.type === 'date' ? '(YYYY-MM-DD)' : '');
}

export interface RunsFlags {
  job: string | undefined;
  status: string | undefined;
  limit: number | undefined;
  offset: number | undefined;
}

export async function listRuns(ctx: Ctx, flags: RunsFlags): Promise<number> {
  const client = await connect(ctx);
  const body = await client.get<RunList>('/runs', {
    query: { job: flags.job, status: flags.status, limit: flags.limit ?? 20, offset: flags.offset },
  });
  const { ui } = ctx;
  if (ui.json) {
    ui.printJson(body);
    return Exit.ok;
  }
  if (body.runs.length === 0) {
    ui.out('No runs.');
    return Exit.ok;
  }
  const rows = body.runs.map((r) => [
    `#${shortId(r.id)}`,
    r.job ?? color('dim', `${r.type}@${r.queue}`),
    statusText(r.status),
    ago(r.enqueued_at),
    duration(r.elapsed_ms),
  ]);
  for (const line of table(['ID', 'JOB', 'STATUS', 'STARTED', 'DURATION'], rows)) ui.out(line);
  const shown = body.offset + body.runs.length;
  if (shown < body.total) {
    ui.info(color('dim', `${shown} of ${body.total}; next page: --offset ${shown}`));
  }
  return Exit.ok;
}

export async function showRun(ctx: Ctx, ref: string): Promise<number> {
  const client = await connect(ctx);
  const r = await client.get<Run>(`/runs/${encodeURIComponent(cleanRef(ref))}`);
  const { ui } = ctx;
  if (ui.json) {
    ui.printJson(r);
    return Exit.ok;
  }
  const rows: [string, string | undefined][] = [
    ['Run', `#${shortId(r.id)} ${color('dim', r.id)}`],
    ['Job', r.job],
    ['Status', statusText(r.status)],
    ['Error', r.error],
    ['Queue', r.queue],
    ['Worker', r.worker_id],
    ['Attempts', `${r.attempts} of ${r.max_retries + 1}`],
    ['Queued', `${r.enqueued_at} (${ago(r.enqueued_at)})`],
    ['Started', r.started_at],
    ['Finished', r.finished_at],
    ['Duration', r.elapsed_ms ? duration(r.elapsed_ms) : undefined],
    ['Panel', runLink(client.url, r)],
  ];
  for (const [k, v] of rows) if (v) ui.out(`${k.padEnd(9)}${v}`);
  const params = Object.entries(r.params ?? {});
  if (params.length > 0) {
    ui.out('Params');
    for (const [k, v] of params) ui.out(`  ${k}=${v}`);
  }
  return Exit.ok;
}

export async function cancel(ctx: Ctx, ref: string): Promise<number> {
  const client = await connect(ctx);
  const id = cleanRef(ref);
  let r: Run;
  try {
    r = (await client.request<Run>('DELETE', `/runs/${encodeURIComponent(id)}`)).data;
  } catch (err) {
    if (err instanceof CliError && err.code === 'conflict') {
      throw new CliError(
        `run #${shortId(id)} has already started. Lute can only cancel a run no worker has taken yet; ` +
          'stopping a running one is not supported yet.',
        Exit.usage,
        { code: 'conflict' },
      );
    }
    throw err;
  }
  if (ctx.ui.json) ctx.ui.printJson(r);
  else ctx.ui.out(`${color('green', '✓')} Cancelled run #${shortId(r.id)}`);
  return Exit.ok;
}

export async function retry(ctx: Ctx, ref: string): Promise<number> {
  const client = await connect(ctx);
  const r = (await client.request<Run>('POST', `/runs/${encodeURIComponent(cleanRef(ref))}/retry`)).data;
  if (ctx.ui.json) {
    ctx.ui.printJson(r);
    return Exit.ok;
  }
  ctx.ui.out(`${color('green', '✓')} Queued run #${shortId(r.id)} again, with the same parameters`);
  const link = runLink(client.url, r);
  if (link) ctx.ui.out(color('dim', `  ${link}`));
  return Exit.ok;
}
