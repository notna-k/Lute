import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import type { Parameter } from '../src/client.ts';
import { CliError } from '../src/output.ts';
import { missingRequired, parseParamFlags, toParams } from '../src/params.ts';

const schema: Parameter[] = [
  { name: 'environment', type: 'select', env_var: 'ENVIRONMENT', required: true, options: [{ value: 'staging' }] },
  { name: 'regions', type: 'multiselect', env_var: 'REGIONS', required: false },
  { name: 'dry_run', type: 'bool', env_var: 'DRY_RUN', required: true, default: false },
  { name: 'notes', type: 'string', env_var: 'NOTES', required: false },
  { name: 'token', type: 'secret', env_var: 'TOKEN', required: true },
];

const usageError = (err: unknown) => err instanceof CliError && err.exit === 2;

describe('parseParamFlags', () => {
  it('splits on the first =, so values may contain =', () => {
    const got = parseParamFlags(['environment=staging', 'notes=a=b']);
    assert.deepEqual([...got], [['environment', 'staging'], ['notes', 'a=b']]);
  });

  it('keeps the last value of a repeated name, and allows an empty value', () => {
    assert.deepEqual([...parseParamFlags(['notes=one', 'notes=two', 'regions='])], [['notes', 'two'], ['regions', '']]);
  });

  it('reads name=@file from the file, without its final newline', () => {
    const got = parseParamFlags(['notes=@release.md'], (p) => {
      assert.equal(p, 'release.md');
      return 'line 1\nline 2\n';
    });
    assert.equal(got.get('notes'), 'line 1\nline 2');
  });

  it('keeps a lone @ as a value', () => {
    assert.equal(parseParamFlags(['notes=@']).get('notes'), '@');
  });

  it('rejects a flag without a name or an =, and a file it cannot read', () => {
    assert.throws(() => parseParamFlags(['environment']), usageError);
    assert.throws(() => parseParamFlags(['=staging']), usageError);
    assert.throws(() => parseParamFlags(['notes=@missing'], () => { throw new Error('ENOENT'); }), usageError);
  });
});

describe('toParams', () => {
  it('turns a multiselect into a list and leaves the rest as strings for core to check', () => {
    const got = toParams(new Map([['regions', 'eu, us,,'], ['dry_run', 'true'], ['environment', 'staging']]), schema);
    assert.deepEqual(got, { regions: ['eu', 'us'], dry_run: 'true', environment: 'staging' });
  });

  it('names a parameter the job does not have, and lists the ones it does', () => {
    assert.throws(
      () => toParams(new Map([['enviroment', 'staging']]), schema),
      (err: unknown) => usageError(err) && (err as CliError).fields?.enviroment !== undefined && /environment, regions/.test((err as Error).message),
    );
  });

  it('refuses a value for a secret, which only resolves on the worker', () => {
    assert.throws(() => toParams(new Map([['token', 'x']]), schema), usageError);
  });
});

describe('missingRequired', () => {
  it('lists required inputs with no value and no default, never secrets', () => {
    assert.deepEqual(missingRequired({}, schema).map((p) => p.name), ['environment']);
    assert.deepEqual(missingRequired({ environment: 'staging' }, schema), []);
  });
});
