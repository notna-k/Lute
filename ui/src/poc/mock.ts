/** Mock job definitions for the PoCs. No backend involved. */
import type { Build, JobDef, ParamValue } from './params/types';

export const MOCK_JOB: JobDef = {
  slug: 'web-release',
  name: 'web-release',
  description: 'Builds and ships the marketing site.',
  queue: 'deploy',
  runtime: 'node:25-alpine',
  command: './scripts/ship.sh',
  source: { repo: 'infra/jobs', path: 'jobs/web-release.yaml', commit: 'a91f0c' },
  params: [
    {
      id: 'p1',
      name: 'environment',
      type: 'select',
      label: 'Target environment',
      env: 'ENVIRONMENT',
      required: true,
      default: 'staging',
      options: [
        { value: 'dev', label: 'Development', hint: 'dev.lute.dev', tone: 'success' },
        { value: 'staging', label: 'Staging', hint: 'staging.lute.dev', tone: 'warning' },
        { value: 'prod', label: 'Production', hint: 'approval required', tone: 'danger' },
      ],
    },
    {
      id: 'p2',
      name: 'regions',
      type: 'multiselect',
      label: 'Regions',
      env: 'REGIONS',
      description: 'Passed to the command as a comma-separated list.',
      minSelected: 1,
      default: ['eu-central', 'us-east'],
      options: [
        { value: 'eu-central' },
        { value: 'eu-west' },
        { value: 'us-east' },
        { value: 'us-west' },
        { value: 'ap-south' },
      ],
    },
    {
      id: 'p3',
      name: 'release_date',
      type: 'date',
      label: 'Release date',
      env: 'RELEASE_DATE',
      default: '2026-07-25',
    },
    {
      id: 'p4',
      name: 'canary_pct',
      type: 'slider',
      label: 'Canary traffic',
      env: 'CANARY_PCT',
      description: 'Share of traffic sent to the new build before full rollout.',
      min: 0,
      max: 100,
      step: 5,
      unit: '%',
      default: 10,
    },
    {
      id: 'p5',
      name: 'git_ref',
      type: 'string',
      label: 'Git ref',
      env: 'GIT_REF',
      description: 'Branch, tag, or commit SHA to build.',
      required: true,
      pattern: '^[\\w./-]+$',
      default: 'main',
    },
    {
      id: 'p6',
      name: 'dry_run',
      type: 'bool',
      label: 'Dry run',
      env: 'DRY_RUN',
      description: 'Produces a plan without shifting any traffic.',
      default: true,
    },
    {
      id: 'p7',
      name: 'deploy_token',
      type: 'secret',
      label: 'Deploy token',
      env: 'DEPLOY_TOKEN',
      secretRef: 'secrets/deploy',
      required: true,
    },
  ],
};

const MIN = 60_000;

export const MOCK_BUILDS: Build[] = [
  {
    id: 412,
    status: 'running',
    startedAt: Date.now() - 2 * MIN,
    by: 'anton',
    summary: 'staging · eu-central,us-east',
  },
  {
    id: 411,
    status: 'passed',
    startedAt: Date.now() - 47 * MIN,
    durationMs: 214_000,
    by: 'cron',
    summary: 'staging · eu-central',
  },
  {
    id: 410,
    status: 'failed',
    startedAt: Date.now() - 96 * MIN,
    durationMs: 61_000,
    by: 'anton',
    summary: 'prod · all regions',
  },
  {
    id: 409,
    status: 'passed',
    startedAt: Date.now() - 180 * MIN,
    durationMs: 198_000,
    by: 'api-key:ci',
    summary: 'dev · eu-west',
  },
  {
    id: 408,
    status: 'passed',
    startedAt: Date.now() - 340 * MIN,
    durationMs: 205_000,
    by: 'anton',
    summary: 'staging · eu-central,us-east',
  },
];

/** The values each past build ran with — the source for "start from". */
export const PAST_VALUES: Record<number, Record<string, ParamValue>> = {
  412: {
    environment: 'staging',
    regions: ['eu-central', 'us-east'],
    release_date: '2026-07-25',
    canary_pct: 10,
    git_ref: 'main',
    dry_run: true,
  },
  411: {
    environment: 'staging',
    regions: ['eu-central'],
    release_date: '2026-07-24',
    canary_pct: 25,
    git_ref: 'main',
    dry_run: false,
  },
  410: {
    environment: 'prod',
    regions: ['eu-central', 'eu-west', 'us-east', 'us-west', 'ap-south'],
    release_date: '2026-07-23',
    canary_pct: 100,
    git_ref: 'v2.4.1',
    dry_run: false,
  },
  409: {
    environment: 'dev',
    regions: ['eu-west'],
    release_date: '2026-07-22',
    canary_pct: 50,
    git_ref: 'feat/new-hero',
    dry_run: true,
  },
};

export function relativeTime(ts: number): string {
  const s = Math.round((Date.now() - ts) / 1000);
  if (s < 60) return `${s}s ago`;
  const m = Math.floor(s / 60);
  if (m < 60) return `${m}m ago`;
  return `${Math.floor(m / 60)}h ago`;
}

export function formatDuration(ms?: number): string {
  if (!ms) return '—';
  const s = Math.round(ms / 1000);
  return `${Math.floor(s / 60)}m ${String(s % 60).padStart(2, '0')}s`;
}
