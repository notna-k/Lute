import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { exitForStatus, redact } from '../src/client.ts';
import { ago, cleanRef, duration, isFinal, runLink, shortId, table } from '../src/format.ts';

describe('exitForStatus', () => {
  it('maps each HTTP failure to the exit code scripts branch on', () => {
    const cases: [number, number][] = [
      [400, 2], [409, 2], [422, 2],
      [401, 3], [403, 3],
      [404, 4],
      [500, 5], [502, 5], [503, 5],
    ];
    for (const [status, exit] of cases) assert.equal(exitForStatus(status), exit, `HTTP ${status}`);
  });
});

describe('redact', () => {
  it('keeps only the public prefix of a key', () => {
    const key = 'lute_sk_abcdefgh12345678secretpart';
    const shown = redact(key);
    assert.ok(shown.startsWith('lute_sk_abcdefgh'));
    assert.ok(!shown.includes('12345678secretpart'));
  });

  it('shows nothing of something that is not a key', () => {
    assert.equal(redact('hunter2'), '(redacted)');
  });
});

describe('format', () => {
  it('shortens ids and accepts the #short form back', () => {
    assert.equal(shortId('66fb1c2d3e4f5a6b7c8d9e0f'), '66fb1c2d');
    assert.equal(cleanRef(' #66fb1c2d '), '66fb1c2d');
  });

  it('knows which statuses are final', () => {
    assert.deepEqual(
      ['pending', 'running', 'done', 'failed', 'dead', 'unknown'].map((s) => isFinal(s as never)),
      [false, false, true, true, true, true],
    );
  });

  it('writes durations and ages for people', () => {
    assert.equal(duration(38_200), '38s');
    assert.equal(duration(125_000), '2m 5s');
    assert.equal(duration(undefined), '—');
    assert.equal(ago(new Date(Date.now() - 90_000).toISOString()), '1m ago');
  });

  it('links a job run to its page in the panel, and a raw run to nothing', () => {
    const run = { id: '66fb1c2d3e4f5a6b7c8d9e0f', job: 'web-release' } as never;
    assert.equal(runLink('https://ci.acme.dev', run), 'https://ci.acme.dev/jobs/web-release/builds/66fb1c2d');
    assert.equal(runLink('https://ci.acme.dev', { id: 'x' } as never), undefined);
  });

  it('pads table columns', () => {
    assert.deepEqual(table(['A', 'B'], [['long cell', 'x'], ['s', 'y']]), ['A          B', 'long cell  x', 's          y']);
  });
});
