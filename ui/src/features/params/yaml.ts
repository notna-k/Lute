/**
 * Job definition → YAML.
 *
 * PRODUCT.md §6: "a Job's YAML is a pure projection of its fields. Nothing in a
 * definition may be expressible only outside the file." The schema editor
 * therefore has exactly one artifact — this text, which you commit to the
 * job-definitions repo. If a control in the editor cannot round-trip to a line
 * here, it does not belong.
 *
 * The emitted keys are the on-disk shape parsed by `yamlJob` in
 * api/internal/jobdefs/sync.go — note `env:` on disk vs `envVar` on the wire.
 */
import { typeDef } from './registry';
import type { JobDefinition, ParameterField, ParameterOption } from '@/types/jobs';

/** Common keys every parameter emits, in order, before its type's own. */
const COMMON: (keyof ParameterField)[] = ['type', 'label', 'description', 'required'];

function scalar(v: unknown): string {
  if (Array.isArray(v)) return `[${v.map(scalar).join(', ')}]`;
  if (typeof v === 'string') {
    if (v === '') return "''";
    // Quote anything YAML would otherwise read as a non-string.
    const bare =
      /^[A-Za-z_][\w./-]*$/.test(v) && !['true', 'false', 'null', 'yes', 'no', 'on', 'off'].includes(v);
    return bare ? v : `'${v.replace(/'/g, "''")}'`;
  }
  return String(v);
}

function optionLine(o: ParameterOption): string {
  const parts = [`value: ${scalar(o.value)}`];
  if (o.label && o.label !== o.value) parts.push(`label: ${scalar(o.label)}`);
  if (o.hint) parts.push(`hint: ${scalar(o.hint)}`);
  if (o.tone && o.tone !== 'neutral') parts.push(`tone: ${o.tone}`);
  return `{ ${parts.join(', ')} }`;
}

function paramLines(field: ParameterField): string[] {
  const out: string[] = [`  - name: ${field.name || 'unnamed'}`];

  for (const key of [...COMMON, ...typeDef(field.type).yamlKeys]) {
    const value = field[key];
    if (value === undefined || value === '' || value === false) continue;
    if (key === 'options') {
      out.push('    options:');
      for (const o of field.options ?? []) out.push(`      - ${optionLine(o)}`);
      continue;
    }
    out.push(`    ${key}: ${scalar(value)}`);
  }
  // `env` sits with identity rather than in COMMON because its on-disk key
  // differs from the wire key.
  if (field.envVar) out.splice(1, 0, `    env: ${field.envVar}`);
  // `default` last: it reads better after the constraints that bound it.
  if (field.default !== undefined && field.default !== '') {
    out.push(`    default: ${scalar(field.default)}`);
  }
  return out;
}

export interface YamlDoc {
  text: string;
  /** 0-based line range per parameter name, so a UI can highlight a block. */
  ranges: Record<string, [number, number]>;
}

export function toYaml(job: JobDefinition, params: ParameterField[] = job.parameters): YamlDoc {
  const lines: string[] = [
    `# ${job.source.repo} · ${job.source.path}`,
    `name: ${job.name}`,
  ];
  if (job.description) lines.push(`description: ${scalar(job.description)}`);
  lines.push(`queue: ${job.queue}`);

  const labels = Object.entries(job.labelSelector ?? {});
  if (labels.length) {
    lines.push('labels:');
    for (const [k, v] of labels) lines.push(`  ${k}: ${scalar(v)}`);
  }

  lines.push(`runtime: ${job.runtime}`, `command: ${scalar(job.command)}`);

  const ranges: Record<string, [number, number]> = {};
  if (params.length) {
    lines.push('parameters:');
    for (const field of params) {
      const start = lines.length;
      lines.push(...paramLines(field));
      ranges[field.name] = [start, lines.length - 1];
    }
  }
  return { text: lines.join('\n'), ranges };
}

/** Offers a YAML document as a file download. */
export function downloadYaml(text: string, filename: string): void {
  const url = URL.createObjectURL(new Blob([`${text}\n`], { type: 'text/yaml' }));
  const a = document.createElement('a');
  a.href = url;
  a.download = filename;
  a.click();
  URL.revokeObjectURL(url);
}

// --- authoring helpers -----------------------------------------------------

let seq = 0;
/** Client-side identity for a draft parameter — survives renames. */
export function newDraftId(): string {
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
