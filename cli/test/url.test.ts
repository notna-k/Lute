import assert from 'node:assert/strict';
import { describe, it } from 'node:test';

import { CliError } from '../src/output.ts';
import { isLoopback, isPlainHttp, normalizeUrl } from '../src/url.ts';

describe('normalizeUrl', () => {
  it('keeps the origin and drops a trailing slash or path', () => {
    assert.equal(normalizeUrl('https://ci.acme.dev/'), 'https://ci.acme.dev');
    assert.equal(normalizeUrl('  https://ci.acme.dev:8443/jobs/x '), 'https://ci.acme.dev:8443');
  });

  it('assumes https when no scheme is typed', () => {
    assert.equal(normalizeUrl('ci.acme.dev'), 'https://ci.acme.dev');
  });

  it('keeps plain http when it is typed', () => {
    assert.equal(normalizeUrl('http://10.0.4.12:8080'), 'http://10.0.4.12:8080');
  });

  it('refuses empty input, other schemes and credentials in the URL', () => {
    for (const bad of ['', '   ', 'ftp://ci.acme.dev', 'https://anton:pw@ci.acme.dev', 'https://']) {
      assert.throws(() => normalizeUrl(bad), (err: unknown) => err instanceof CliError && err.exit === 2, bad);
    }
  });
});

describe('plain HTTP', () => {
  it('is plain for a network address over http', () => {
    assert.equal(isPlainHttp('http://10.0.4.12:8080'), true);
    assert.equal(isPlainHttp('http://ci.acme.dev'), true);
  });

  it('is fine over https', () => {
    assert.equal(isPlainHttp('https://ci.acme.dev'), false);
  });

  it('is exempt for localhost, 127.0.0.1 and ::1, where the key never leaves the machine', () => {
    for (const url of ['http://localhost:8080', 'http://127.0.0.1:8080', 'http://[::1]:8080', 'http://127.1.2.3']) {
      assert.equal(isLoopback(url), true, url);
      assert.equal(isPlainHttp(url), false, url);
    }
  });

  it('does not mistake a look-alike host for localhost', () => {
    for (const url of ['http://localhost.evil.dev', 'http://127.0.0.1.nip.io', 'http://10.0.0.1']) {
      assert.equal(isLoopback(url), false, url);
    }
  });
});
