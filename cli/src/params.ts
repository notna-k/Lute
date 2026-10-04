// -p name=value flags, turned into the params a job run takes. Core validates the values;
// this only shapes them by the job's schema and catches names the job does not have.

import { readFileSync } from 'node:fs';

import type { Parameter } from './client.ts';
import { CliError, Exit } from './output.ts';

/**
 * parseParamFlags reads repeated "-p name=value" flags. "name=@path" reads the value
 * from a file, without its final newline. A name given twice keeps the last value.
 */
export function parseParamFlags(flags: string[], readFile = (p: string) => readFileSync(p, 'utf8')): Map<string, string> {
  const out = new Map<string, string>();
  for (const flag of flags) {
    const eq = flag.indexOf('=');
    if (eq <= 0) {
      throw new CliError(`-p ${flag}: expected name=value`, Exit.usage);
    }
    const name = flag.slice(0, eq).trim();
    let value = flag.slice(eq + 1);
    if (value.startsWith('@') && value.length > 1) {
      const path = value.slice(1);
      try {
        value = readFile(path).replace(/\r?\n$/, '');
      } catch (err) {
        throw new CliError(`-p ${name}=@${path}: ${(err as Error).message}`, Exit.usage);
      }
    }
    out.set(name, value);
  }
  return out;
}

/**
 * toParams shapes raw values by the schema: a multiselect becomes a list from "a,b";
 * everything else stays a string, which core coerces and checks.
 */
export function toParams(raw: Map<string, string>, schema: Parameter[]): Record<string, unknown> {
  const byName = new Map(schema.map((p) => [p.name, p]));
  const out: Record<string, unknown> = {};
  for (const [name, value] of raw) {
    const param = byName.get(name);
    if (!param) {
      const known = schema.map((p) => p.name).join(', ') || 'none';
      throw new CliError(`the job has no parameter "${name}" (it takes: ${known})`, Exit.usage, {
        code: 'validation_failed',
        fields: { [name]: 'unknown parameter' },
      });
    }
    if (param.type === 'secret') {
      throw new CliError(`"${name}" is a secret; it resolves on the worker and cannot be passed`, Exit.usage, {
        code: 'validation_failed',
        fields: { [name]: 'secrets cannot be passed' },
      });
    }
    out[name] =
      param.type === 'multiselect'
        ? value
            .split(',')
            .map((v) => v.trim())
            .filter(Boolean)
        : value;
  }
  return out;
}

/** missingRequired lists required inputs that have no value and no default. */
export function missingRequired(params: Record<string, unknown>, schema: Parameter[]): Parameter[] {
  return schema.filter((p) => p.required && p.type !== 'secret' && p.default === undefined && !(p.name in params));
}
