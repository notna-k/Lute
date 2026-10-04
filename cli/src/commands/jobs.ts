// lute jobs / lute jobs show <slug>.

import type { JobDetail, JobSummary, Parameter } from '../client.ts';
import { connect, type Ctx } from '../context.ts';
import { ago, shortId, statusText, table } from '../format.ts';
import { color, Exit } from '../output.ts';

export async function listJobs(ctx: Ctx): Promise<number> {
  const client = await connect(ctx);
  const body = await client.get<{ jobs: JobSummary[] }>('/jobs');
  const { ui } = ctx;
  if (ui.json) {
    ui.printJson(body);
    return Exit.ok;
  }
  if (body.jobs.length === 0) {
    ui.out('No jobs yet. Definitions come from YAML files in Git; see "Sync from Git" in the panel.');
    return Exit.ok;
  }
  const rows = body.jobs.map((j) => [
    j.slug,
    j.queue,
    j.last_run ? `${statusText(j.last_run.status)} #${shortId(j.last_run.id)} ${color('dim', ago(j.last_run.enqueued_at))}` : color('dim', 'never run'),
  ]);
  for (const line of table(['SLUG', 'QUEUE', 'LAST RUN'], rows)) ui.out(line);
  return Exit.ok;
}

export async function showJob(ctx: Ctx, slug: string): Promise<number> {
  const client = await connect(ctx);
  const job = await client.get<JobDetail>(`/jobs/${encodeURIComponent(slug)}`);
  const { ui } = ctx;
  if (ui.json) {
    ui.printJson(job);
    return Exit.ok;
  }
  ui.out(`${color('bold', job.name)} ${color('dim', `(${job.slug})`)}`);
  if (job.description) ui.out(job.description);
  ui.out();
  ui.out(`Queue    ${job.queue}`);
  ui.out(`Runtime  ${job.runtime}`);
  ui.out(`Command  ${job.command}`);
  if (job.source_repo) ui.out(`Source   ${job.source_repo}`);
  if (job.last_run) ui.out(`Last run ${statusText(job.last_run.status)} #${shortId(job.last_run.id)} ${ago(job.last_run.enqueued_at)}`);
  ui.out();
  if (job.parameters.length === 0) {
    ui.out(`No parameters: lute run ${job.slug}`);
    return Exit.ok;
  }
  ui.out('Parameters');
  for (const p of job.parameters) {
    for (const line of describeParam(p)) ui.out(line);
  }
  const example = job.parameters
    .filter((p) => p.required && p.default === undefined && p.type !== 'secret')
    .map((p) => ` -p ${p.name}=${p.options?.[0]?.value ?? `<${p.type}>`}`)
    .join('');
  ui.out();
  ui.out(color('dim', `lute run ${job.slug}${example}`));
  return Exit.ok;
}

export function describeParam(p: Parameter): string[] {
  const traits = [p.type, p.required ? 'required' : 'optional'];
  if (p.default !== undefined) traits.push(`default ${JSON.stringify(p.default)}`);
  const lines = [`  ${color('bold', p.name)}  ${color('dim', traits.join(' · '))}`];
  if (p.label && p.label !== p.name) lines.push(`      ${p.label}${p.description ? ` — ${p.description}` : ''}`);
  else if (p.description) lines.push(`      ${p.description}`);
  if (p.options?.length) {
    const opts = p.options.map((o) => (o.label && o.label !== o.value ? `${o.value} (${o.label})` : o.value));
    lines.push(`      ${p.type === 'multiselect' ? 'any of' : 'one of'}: ${opts.join(', ')}`);
  }
  if (p.type === 'secret') lines.push('      resolved on the worker; it cannot be passed');
  return lines;
}
