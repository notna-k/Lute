import type { JobDefinition } from '@/types/jobs';

export type Health = 'all' | 'failing' | 'running' | 'drift';
export type SortKey = 'name' | 'recent' | 'slowest' | 'flakiest';

export interface JobGroup {
  folder: string;
  rows: JobDefinition[];
}

/** The folder a definition lives in: `jobdefs/release/api.yaml` → `jobdefs/release`. */
export function folderOf(job: JobDefinition): string {
  const parts = (job.source.path ?? '').split('/').filter(Boolean);
  parts.pop();
  return parts.length ? parts.join('/') : 'root';
}

export function matchesHealth(job: JobDefinition, health: Health): boolean {
  switch (health) {
    case 'failing':
      return job.lastBuild?.status === 'failed';
    case 'running':
      return job.lastBuild?.status === 'running' || job.lastBuild?.status === 'queued';
    case 'drift':
      return job.gitState !== 'synced';
    default:
      return true;
  }
}

function matchesQuery(job: JobDefinition, needle: string): boolean {
  return (
    !needle ||
    job.name.toLowerCase().includes(needle) ||
    job.slug.toLowerCase().includes(needle) ||
    job.source.path.toLowerCase().includes(needle)
  );
}

const COMPARE: Record<SortKey, (a: JobDefinition, b: JobDefinition) => number> = {
  name: (a, b) => a.name.localeCompare(b.name),
  recent: (a, b) => (b.lastBuild?.startedAt ?? 0) - (a.lastBuild?.startedAt ?? 0),
  slowest: (a, b) => b.medianDurationMs - a.medianDurationMs,
  flakiest: (a, b) => a.successRate - b.successRate,
};

/** Filters jobs, then groups them by folder; `sort` orders rows within each group. */
export function groupJobs(
  jobs: JobDefinition[],
  filter: { query: string; health: Health; folders: string[]; queues: string[]; sort: SortKey },
): JobGroup[] {
  const needle = filter.query.trim().toLowerCase();
  const byFolder = new Map<string, JobDefinition[]>();
  for (const job of jobs) {
    const folder = folderOf(job);
    if (
      !matchesHealth(job, filter.health) ||
      (filter.folders.length > 0 && !filter.folders.includes(folder)) ||
      (filter.queues.length > 0 && !filter.queues.includes(job.queue)) ||
      !matchesQuery(job, needle)
    ) {
      continue;
    }
    byFolder.set(folder, [...(byFolder.get(folder) ?? []), job]);
  }
  const compare = COMPARE[filter.sort] ?? COMPARE.name;
  return [...byFolder.entries()]
    .map(([folder, rows]) => ({ folder, rows: rows.sort(compare) }))
    .sort((a, b) => a.folder.localeCompare(b.folder));
}
