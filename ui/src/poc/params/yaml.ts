/**
 * Schema → YAML.
 *
 * PRODUCT.md §6: "a Job's YAML is a pure projection of its fields. Nothing in a
 * definition may be expressible only outside the file." The builder therefore
 * has exactly one artifact — this text — and every PoC shows it. If a control
 * in the editor cannot round-trip to a line here, it does not belong.
 */
import { typeDef } from './registry';
import type { JobDef, ParamSpec, ParamValue } from './types';

/** Common keys every type emits, before its own `yamlKeys`. */
const COMMON: (keyof ParamSpec)[] = ['type', 'label', 'env', 'description', 'required'];

function scalar(v: unknown): string {
  if (Array.isArray(v)) return `[${v.map(scalar).join(', ')}]`;
  if (typeof v === 'string') {
    // Quote anything YAML would otherwise read as a non-string.
    const bare = /^[A-Za-z_][\w./-]*$/.test(v) && !['true', 'false', 'null', 'yes', 'no'].includes(v);
    return bare || v === '' ? (v === '' ? "''" : v) : `'${v.replace(/'/g, "''")}'`;
  }
  return String(v);
}

function optionLine(o: { value: string; label?: string; hint?: string; tone?: string }): string {
  const parts = [`value: ${scalar(o.value)}`];
  if (o.label && o.label !== o.value) parts.push(`label: ${scalar(o.label)}`);
  if (o.hint) parts.push(`hint: ${scalar(o.hint)}`);
  if (o.tone && o.tone !== 'neutral') parts.push(`tone: ${o.tone}`);
  return `{ ${parts.join(', ')} }`;
}

function paramLines(spec: ParamSpec): string[] {
  const out: string[] = [`  - name: ${spec.name || 'unnamed'}`];
  const keys = [...COMMON, ...typeDef(spec.type).yamlKeys];

  for (const key of keys) {
    const value = spec[key] as unknown;
    if (value === undefined || value === '' || value === false) continue;
    if (key === 'options') {
      out.push('    options:');
      for (const o of spec.options ?? []) out.push(`      - ${optionLine(o)}`);
      continue;
    }
    out.push(`    ${key}: ${scalar(value)}`);
  }
  // `default` last: it reads better after the constraints that bound it.
  if (spec.default !== undefined && spec.default !== '') {
    out.push(`    default: ${scalar(spec.default)}`);
  }
  return out;
}

export interface YamlDoc {
  text: string;
  /** 0-based line range per param id, so a UI can highlight the selected field. */
  ranges: Record<string, [number, number]>;
}

export function toYaml(job: JobDef): YamlDoc {
  const lines: string[] = [
    `# ${job.source.repo} · ${job.source.path}`,
    `name: ${job.name}`,
  ];
  if (job.description) lines.push(`description: ${scalar(job.description)}`);
  lines.push(`queue: ${job.queue}`, `runtime: ${job.runtime}`, `command: ${scalar(job.command)}`);

  const ranges: Record<string, [number, number]> = {};
  if (job.params.length) {
    lines.push('parameters:');
    for (const spec of job.params) {
      const start = lines.length;
      lines.push(...paramLines(spec));
      ranges[spec.id] = [start, lines.length - 1];
    }
  }
  return { text: lines.join('\n'), ranges };
}

/** The `docker run` env block, for the "what will actually happen" preview. */
export function toEnvScript(
  pairs: { key: string; value: string; masked: boolean }[],
  command: string
): string {
  const env = pairs
    .map((p) => `  -e ${p.key}=${p.masked ? '$(secret)' : JSON.stringify(p.value)} \\`)
    .join('\n');
  return `docker run --rm \\\n${env}\n  ${command}`;
}

// --- authoring helpers -----------------------------------------------------

let seq = 0;
export function newId(): string {
  seq += 1;
  return `p${Date.now().toString(36)}${seq}`;
}

/** `release date` → `RELEASE_DATE`. Keeps env vars conventional without asking. */
export function envFromName(name: string): string {
  return name
    .replace(/[^A-Za-z0-9]+/g, '_')
    .replace(/([a-z0-9])([A-Z])/g, '$1_$2')
    .toUpperCase()
    .replace(/^_+|_+$/g, '');
}

/** `Release date` → `release_date`. */
export function nameFromLabel(label: string): string {
  return label
    .trim()
    .toLowerCase()
    .replace(/[^a-z0-9]+/g, '_')
    .replace(/^_+|_+$/g, '');
}

export function defaultIsValidFor(spec: ParamSpec, value: ParamValue): boolean {
  return typeDef(spec.type).validate?.(spec, value) === null;
}
