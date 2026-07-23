/**
 * Job-definition service.
 *
 * NOTE: the Core endpoints for Git-managed job definitions are not built yet
 * (see PRODUCT.md — GitOps sync + parameter schema). Until they land, this
 * module serves static fixtures shaped exactly like the future API responses,
 * so the Console UI is wired against the real types. Swap the bodies for
 * `api.get(...)` calls once the backend exists; the signatures stay the same.
 */

import type { Build, JobDefinition } from '@/types/jobs';

const JOBS: JobDefinition[] = [
  {
    slug: 'web-release',
    name: 'web-release',
    description:
      'Builds and ships the marketing site. Definition is synced from Git — trigger and observe here, edit config in the repo.',
    queue: 'deploy',
    labelSelector: { region: 'eu' },
    runtime: 'node:25-alpine',
    command: './scripts/ship.sh',
    source: {
      repo: 'infra/jobs',
      path: 'deploy/web-release.yaml',
      commit: 'a91f0c',
      inSync: true,
    },
    successRate: 0.98,
    medianDurationMs: 72_000,
    parameters: [
      {
        name: 'environment',
        type: 'select',
        label: 'Target environment',
        envVar: 'ENVIRONMENT',
        required: true,
        default: 'staging',
        options: [
          { value: 'dev', label: 'Development', hint: 'dev.lute.dev', tone: 'success' },
          { value: 'staging', label: 'Staging', hint: 'staging.lute.dev', tone: 'warning' },
          { value: 'prod', label: 'Production', hint: 'www.lute.dev · approval required', tone: 'danger' },
        ],
      },
      {
        name: 'regions',
        type: 'multiselect',
        label: 'Regions',
        envVar: 'REGIONS',
        description: 'Passed to the command as a comma-separated list.',
        default: ['eu-central', 'us-east'],
        options: [
          { value: 'eu-central', label: 'eu-central' },
          { value: 'eu-west', label: 'eu-west' },
          { value: 'us-east', label: 'us-east' },
          { value: 'us-west', label: 'us-west' },
          { value: 'ap-south', label: 'ap-south' },
        ],
      },
      {
        name: 'release_date',
        type: 'date',
        label: 'Release date',
        envVar: 'RELEASE_DATE',
        default: '2026-07-25',
      },
      {
        name: 'dry_run',
        type: 'bool',
        label: 'Dry run',
        envVar: 'DRY_RUN',
        description: 'Produces a plan without shifting any traffic.',
        default: true,
      },
      {
        name: 'deploy_token',
        type: 'secret',
        label: 'Deploy token',
        envVar: 'DEPLOY_TOKEN',
        secretRef: 'secrets/deploy',
      },
    ],
  },
  {
    slug: 'nightly-etl',
    name: 'nightly-etl',
    description:
      'Runs the analytics rollup and loads warehouse tables. Scheduled nightly; can be triggered on demand.',
    queue: 'batch',
    labelSelector: { class: 'cpu' },
    runtime: 'python:3.12-slim',
    command: 'python -m etl.run',
    source: {
      repo: 'infra/jobs',
      path: 'data/nightly-etl.yaml',
      commit: 'a91f0c',
      inSync: true,
    },
    successRate: 0.94,
    medianDurationMs: 512_000,
    parameters: [
      {
        name: 'window',
        type: 'select',
        label: 'Rollup window',
        envVar: 'WINDOW',
        required: true,
        default: 'day',
        options: [
          { value: 'hour', label: 'Hourly' },
          { value: 'day', label: 'Daily' },
          { value: 'week', label: 'Weekly' },
        ],
      },
      {
        name: 'backfill_from',
        type: 'date',
        label: 'Backfill from',
        envVar: 'BACKFILL_FROM',
      },
      {
        name: 'full_refresh',
        type: 'bool',
        label: 'Full refresh',
        envVar: 'FULL_REFRESH',
        default: false,
      },
    ],
  },
];

const BUILDS: Build[] = [
  { id: '4127', jobSlug: 'web-release', status: 'running', environment: 'staging', startedAt: Date.now() - 20_000 },
  { id: '4126', jobSlug: 'web-release', status: 'passed', environment: 'staging', startedAt: Date.now() - 4 * 60_000, durationMs: 82_000 },
  { id: '4125', jobSlug: 'web-release', status: 'failed', environment: 'prod', startedAt: Date.now() - 60 * 60_000, durationMs: 38_000 },
  { id: '4124', jobSlug: 'web-release', status: 'passed', environment: 'dev', startedAt: Date.now() - 2 * 60 * 60_000, durationMs: 64_000 },
  { id: '4123', jobSlug: 'web-release', status: 'passed', environment: 'staging', startedAt: Date.now() - 3 * 60 * 60_000, durationMs: 70_000 },
  { id: '4122', jobSlug: 'web-release', status: 'passed', environment: 'staging', startedAt: Date.now() - 5 * 60 * 60_000, durationMs: 75_000 },
];

function delay<T>(value: T): Promise<T> {
  return new Promise((resolve) => setTimeout(() => resolve(value), 120));
}

export function listJobs(): Promise<JobDefinition[]> {
  return delay(JOBS);
}

export function getJob(slug: string): Promise<JobDefinition | undefined> {
  return delay(JOBS.find((j) => j.slug === slug));
}

export function listBuilds(slug: string): Promise<Build[]> {
  return delay(BUILDS.filter((b) => b.jobSlug === slug));
}
