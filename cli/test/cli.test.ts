// The lute process end to end against a stand-in for Lute: the output contract (exit
// codes, --json, stdout vs stderr), plain-HTTP consent, and where the key is kept.

import assert from 'node:assert/strict';
import { readFileSync, statSync, writeFileSync } from 'node:fs';
import { networkInterfaces } from 'node:os';
import { join } from 'node:path';
import { after, before, describe, it } from 'node:test';

import { aRun, FakeLute, freshConfigDir, jsonLines, KEY, lute, RUN_ID, webRelease } from './helpers.ts';

const env = (url: string) => ({ LUTE_URL: url, LUTE_API_KEY: KEY });

function lanAddress(): string | undefined {
  for (const addrs of Object.values(networkInterfaces())) {
    for (const a of addrs ?? []) if (a.family === 'IPv4' && !a.internal) return a.address;
  }
  return undefined;
}

describe('connecting', () => {
  let api: FakeLute;
  before(async () => {
    api = await new FakeLute()
      .on('GET', '/whoami', () => ({ body: { key: { id: 'k1', name: 'laptop', prefix: 'lute_sk_abcdefgh', scope: 'account' }, user: { id: 'u1', email: 'anton@acme.dev' } } }))
      .on('GET', '/runs', () => ({ body: { runs: [], total: 0, offset: 0, limit: 20 } }))
      .start();
  });
  after(() => api.stop());

  it('logs in with a piped key, keeps only the URL in config.json and the key in a 0600 file', async () => {
    const dir = freshConfigDir();
    const res = await lute(['login', '--url', api.url, '--api-key-stdin', '--insecure-storage'], { configDir: dir, stdin: `${KEY}\n` });
    assert.equal(res.code, 0, res.stderr);
    assert.match(res.stdout, /Connected to Lute 0\.9\.0 as anton@acme\.dev/);

    const config = readFileSync(join(dir, 'config.json'), 'utf8');
    assert.ok(!config.includes(KEY), 'config.json holds the key');
    assert.equal(JSON.parse(config).url, api.url);
    const creds = join(dir, 'credentials.json');
    assert.equal(statSync(creds).mode & 0o777, 0o600);
    assert.ok(readFileSync(creds, 'utf8').includes(KEY));

    // Later commands use the saved login with no environment at all.
    const runs = await lute(['runs', '--json'], { configDir: dir });
    assert.equal(runs.code, 0, runs.stderr);
    assert.deepEqual(JSON.parse(runs.stdout).runs, []);
  });

  it('stops a login without a keychain and explains, saving nothing', async () => {
    const dir = freshConfigDir();
    const res = await lute(['login', '--url', api.url, '--api-key-stdin'], { configDir: dir, stdin: KEY });
    assert.equal(res.code, 2);
    assert.match(res.stderr, /no system keychain/);
    assert.match(res.stderr, /--insecure-storage/);
    assert.throws(() => statSync(join(dir, 'config.json')));
  });

  it('checks the key before saving it: a refused key exits 3 and saves nothing', async () => {
    const dir = freshConfigDir();
    const res = await lute(['login', '--url', api.url, '--api-key-stdin', '--insecure-storage'], { configDir: dir, stdin: 'lute_sk_wrongwrongwrongwrong' });
    assert.equal(res.code, 3);
    assert.match(res.stderr, /wrong or revoked/);
    assert.throws(() => statSync(join(dir, 'config.json')));
  });

  it('refuses something that is not a Lute key before sending it', async () => {
    const before = api.seen.length;
    const res = await lute(['login', '--url', api.url, '--api-key-stdin', '--insecure-storage'], { stdin: 'ghp_notalutekey' });
    assert.equal(res.code, 2);
    assert.equal(api.seen.length, before, 'the key was sent');
  });

  it('never sends a saved key to another URL', async () => {
    const dir = freshConfigDir();
    await lute(['login', '--url', api.url, '--api-key-stdin', '--insecure-storage'], { configDir: dir, stdin: KEY });
    const other = await new FakeLute().start();
    try {
      const res = await lute(['runs', '--url', other.url], { configDir: dir });
      assert.equal(res.code, 3, res.stderr);
      assert.ok(other.seen.every((s) => s.headers.authorization === undefined), 'the saved key reached another server');
    } finally {
      await other.stop();
    }
  });

  it('logs out: the key and the URL are gone from this machine', async () => {
    const dir = freshConfigDir();
    await lute(['login', '--url', api.url, '--api-key-stdin', '--insecure-storage'], { configDir: dir, stdin: KEY });
    const res = await lute(['logout'], { configDir: dir });
    assert.equal(res.code, 0);
    assert.throws(() => statSync(join(dir, 'config.json')));
    assert.throws(() => statSync(join(dir, 'credentials.json')));
    assert.equal((await lute(['runs'], { configDir: dir })).code, 3);
  });

  it('works from LUTE_URL and LUTE_API_KEY alone, leaving nothing on disk', async () => {
    const dir = freshConfigDir();
    const res = await lute(['whoami'], { configDir: dir, env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    assert.match(res.stdout, /anton@acme\.dev/);
    assert.ok(!res.stdout.includes(KEY) && !res.stderr.includes(KEY), 'whoami printed the key');
    assert.throws(() => statSync(join(dir, 'config.json')));
  });

  it('exits 3 with nothing configured', async () => {
    const res = await lute(['runs', '--json']);
    assert.equal(res.code, 3);
    assert.equal(JSON.parse(res.stderr).error.code, 'unauthorized');
    assert.equal(res.stdout, '');
  });

  it('redacts the key in --debug output', async () => {
    const res = await lute(['runs', '--debug'], { env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    assert.match(res.stderr, /→ GET .*\/api\/public\/v1\/runs/);
    assert.match(res.stderr, /Authorization: Bearer lute_sk_abcdefgh…\(redacted\)/);
    assert.ok(!res.stderr.includes(KEY));
  });
});

describe('plain HTTP', () => {
  it('refuses a plain-HTTP URL from the environment before sending anything (exit 2)', async () => {
    const res = await lute(['runs', '--json'], { env: env('http://10.255.255.1:9') });
    assert.equal(res.code, 2);
    const err = JSON.parse(res.stderr).error;
    assert.match(err.message, /plain HTTP/);
    assert.match(err.message, /--allow-insecure-http/);
  });

  it('refuses a plain-HTTP login without a terminal, before the key is read', async () => {
    const res = await lute(['login', '--url', 'http://10.255.255.1:9', '--api-key-stdin', '--insecure-storage'], { stdin: KEY });
    assert.equal(res.code, 2);
    assert.match(res.stderr, /nothing was sent/);
  });

  it('exempts localhost: no warning, no flag needed', async () => {
    const api = await new FakeLute().on('GET', '/runs', () => ({ body: { runs: [], total: 0, offset: 0, limit: 20 } })).start();
    try {
      const res = await lute(['runs'], { env: env(api.url) });
      assert.equal(res.code, 0, res.stderr);
      assert.ok(!res.stderr.includes('plain HTTP'), res.stderr);
    } finally {
      await api.stop();
    }
  });

  it('with --allow-insecure-http sends it, warns on stderr and keeps --json stdout clean', async (t) => {
    const lan = lanAddress();
    if (!lan) return t.skip('no network address to stand for a remote host');
    const api = await new FakeLute()
      .on('GET', '/runs', () => ({ body: { runs: [], total: 0, offset: 0, limit: 20 } }))
      .on('GET', '/whoami', () => ({ body: { key: { id: 'k1', name: 'ci', prefix: 'lute_sk_abcdefgh', scope: 'service' } } }))
      .start('0.0.0.0');
    const url = api.url.replace('0.0.0.0', lan);
    try {
      const res = await lute(['runs', '--json', '--allow-insecure-http'], { env: env(url) });
      assert.equal(res.code, 0, res.stderr);
      assert.match(res.stderr, /⚠ plain HTTP/);
      assert.deepEqual(JSON.parse(res.stdout).runs, []);

      // Consent given at login is saved with the URL, and the notice still shows.
      const dir = freshConfigDir();
      const login = await lute(['login', '--url', url, '--allow-insecure-http', '--api-key-stdin', '--insecure-storage'], {
        configDir: dir,
        stdin: KEY,
        env: { LUTE_URL: undefined },
      });
      assert.equal(login.code, 0, login.stderr);
      const later = await lute(['runs'], { configDir: dir });
      assert.equal(later.code, 0, later.stderr);
      assert.match(later.stderr, /⚠ plain HTTP/);
    } finally {
      await api.stop();
    }
  });
});

describe('errors and exit codes', () => {
  let api: FakeLute;
  before(async () => {
    api = await new FakeLute()
      .on('GET', /^\/runs\/[0-9a-f]+$/, (s) => (s.path.endsWith('00000000') ? { status: 404, body: { error: { code: 'not_found', message: 'run not found' } } } : undefined))
      .on('GET', '/jobs', () => ({ status: 503, body: { error: { code: 'unavailable', message: 'database is restarting' } } }))
      .on('DELETE', /^\/runs\//, () => ({ status: 409, body: { error: { code: 'conflict', message: 'can only cancel pending jobs' } } }))
      .start();
  });
  after(() => api.stop());

  it('exits 4 for a missing run, with the API error shape on stderr under --json', async () => {
    const res = await lute(['runs', 'show', '00000000', '--json'], { env: env(api.url) });
    assert.equal(res.code, 4);
    assert.deepEqual(JSON.parse(res.stderr), { error: { code: 'not_found', message: 'run not found' } });
    assert.equal(res.stdout, '');
  });

  it('exits 5 when Lute fails or cannot be reached', async () => {
    assert.equal((await lute(['jobs'], { env: env(api.url) })).code, 5);
    const closed = await lute(['jobs'], { env: env('http://127.0.0.1:1') });
    assert.equal(closed.code, 5);
    assert.match(closed.stderr, /could not reach Lute/);
  });

  it('exits 2 for bad usage', async () => {
    for (const args of [['frobnicate'], ['run'], ['runs', '--limit', 'ten'], ['logs', 'a', 'b'], ['run', 'x', '--bogus']]) {
      const res = await lute(args, { env: env(api.url) });
      assert.equal(res.code, 2, `${args.join(' ')}: ${res.stderr}`);
    }
  });

  it('explains that a started run cannot be cancelled yet (exit 2)', async () => {
    const res = await lute(['cancel', '#66fb1c2d'], { env: env(api.url) });
    assert.equal(res.code, 2);
    assert.match(res.stderr, /already started/);
  });

  it('prints no colour without a terminal', async () => {
    const res = await lute(['--help'], { env: { NO_COLOR: undefined } });
    assert.ok(!res.stdout.includes('\x1b['));
  });
});

describe('running a job', () => {
  let api: FakeLute;
  let polls = 0;
  let outcome: 'done' | 'dead' = 'done';
  const logLines = ['> ship@1.0.0 build', 'vite building...', 'built in 4.12s'];

  before(async () => {
    api = await new FakeLute()
      .on('GET', '/jobs/web-release', () => ({ body: webRelease }))
      .on('POST', '/jobs/web-release/runs', (s) => ({
        status: s.headers['idempotency-key'] === 'seen-before' ? 200 : 201,
        body: aRun({ params: s.body }),
      }))
      .on('GET', /^\/runs\/66fb1c2d[0-9a-f]*$/, () => {
        polls++;
        if (polls < 3) return { body: aRun({ status: polls === 1 ? 'pending' : 'running' }) };
        return { body: aRun({ status: outcome, elapsed_ms: 38_000, error: outcome === 'dead' ? 'exit status 9' : undefined }) };
      })
      .on('GET', `/runs/${RUN_ID}/logs`, (s) => {
        // Before a worker takes the run there is no log yet.
        if (polls < 2) return { status: 404, body: { error: { code: 'not_found', message: 'no worker has run this job yet' } } };
        const from = Number(s.query.get('cursor') ?? 0);
        const visible = polls < 3 ? 1 : logLines.length;
        return { body: { lines: logLines.slice(from, visible), direction: 'head', has_more: false, file_size: 100, next_cursor: String(visible) } };
      })
      .start();
  });
  after(() => api.stop());

  const reset = (o: 'done' | 'dead' = 'done') => {
    polls = 0;
    outcome = o;
  };
  const lastPost = () => api.seen.filter((s) => s.method === 'POST').at(-1);

  it('sends -p values shaped by the schema: lists split, files read', async () => {
    reset();
    const dir = freshConfigDir();
    const notes = join(dir, 'notes.md');
    writeFileSync(notes, 'Ship it\n');
    const res = await lute(['run', 'web-release', '-p', 'environment=staging', '-p', 'regions=eu,us', '-p', `notes=@${notes}`, '--json'], { env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    assert.deepEqual(lastPost()?.body, { params: { environment: 'staging', regions: ['eu', 'us'], notes: 'Ship it' } });
    assert.equal(JSON.parse(res.stdout).id, RUN_ID);
  });

  it('rejects an unknown parameter before starting anything (exit 2)', async () => {
    const posts = api.seen.filter((s) => s.method === 'POST').length;
    const res = await lute(['run', 'web-release', '-p', 'enviroment=staging', '--json'], { env: env(api.url) });
    assert.equal(res.code, 2);
    assert.equal(JSON.parse(res.stderr).error.fields.enviroment, 'unknown parameter');
    assert.equal(api.seen.filter((s) => s.method === 'POST').length, posts);
  });

  it('passes --idempotency-key as the Idempotency-Key header', async () => {
    const res = await lute(['run', 'web-release', '-p', 'environment=prod', '--idempotency-key', 'seen-before'], { env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    assert.equal(lastPost()?.headers['idempotency-key'], 'seen-before');
    assert.match(res.stdout, /Found earlier run #66fb1c2d/);
  });

  it('prints the queued run and its panel link', async () => {
    const res = await lute(['run', 'web-release', '-p', 'environment=prod'], { env: env(api.url) });
    assert.match(res.stdout, /Queued run #66fb1c2d on queue deploy/);
    assert.ok(res.stdout.includes(`${api.url}/jobs/web-release/builds/66fb1c2d`));
  });

  it('--wait --json prints one document, the final run, and exits 0 when it passed', async () => {
    reset('done');
    const res = await lute(['run', 'web-release', '-p', 'environment=prod', '--wait', '--json'], { env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    const run = JSON.parse(res.stdout);
    assert.equal(run.status, 'done');
    assert.equal(res.stderr, '', 'progress leaked to stderr under --json');
  });

  it('--wait exits 1 when the run failed', async () => {
    reset('dead');
    const res = await lute(['run', 'web-release', '-p', 'environment=prod', '--wait'], { env: env(api.url) });
    assert.equal(res.code, 1);
    assert.match(res.stdout, /Run #66fb1c2d failed: exit status 9/);
  });

  it('--follow streams the log to stdout, every line once, then the outcome', async () => {
    reset('done');
    const res = await lute(['run', 'web-release', '-p', 'environment=prod', '--follow'], { env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    assert.deepEqual(res.stdout.trim().split('\n'), [...logLines, '✓ Run #66fb1c2d passed in 38s']);
    assert.match(res.stderr, /Queued run #66fb1c2d/);
  });

  it('--follow --json is NDJSON: one {"line"} per line, then {"run"}', async () => {
    reset('dead');
    const res = await lute(['run', 'web-release', '-p', 'environment=prod', '--follow', '--json'], { env: env(api.url) });
    assert.equal(res.code, 1);
    const records = jsonLines(res.stdout) as Record<string, unknown>[];
    assert.deepEqual(records.slice(0, -1), logLines.map((line) => ({ line })));
    assert.equal((records.at(-1)?.run as { status: string }).status, 'dead');
  });

  it('logs -f follows an existing run by its short id', async () => {
    reset('done');
    const res = await lute(['logs', '#66fb1c2d', '-f', '--json'], { env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    const records = jsonLines(res.stdout) as Record<string, unknown>[];
    assert.equal(records.filter((r) => 'line' in r).length, logLines.length);
  });
});

describe('reading', () => {
  let api: FakeLute;
  const lines = Array.from({ length: 1200 }, (_, i) => `line ${i + 1}`);

  before(async () => {
    api = await new FakeLute()
      .on('GET', '/jobs', () => ({ body: { jobs: [{ slug: 'web-release', name: 'Web release', queue: 'deploy', runtime: 'node', last_run: aRun({ status: 'done' }) }] } }))
      .on('GET', '/jobs/web-release', () => ({ body: webRelease }))
      .on('GET', '/runs', (s) => ({ body: { runs: [aRun({ status: s.query.get('status') ?? 'done' })], total: 1, offset: 0, limit: 20, echo: Object.fromEntries(s.query) } }))
      .on('GET', /^\/runs\/[0-9a-f]+$/, () => ({ body: aRun({ status: 'done' }) }))
      .on('GET', /^\/runs\/[0-9a-f]+\/logs$/, (s) => {
        // The real paging: head moves forward from the cursor, tail moves back from it.
        const limit = Number(s.query.get('limit'));
        const cursor = s.query.get('cursor');
        if (s.query.get('direction') === 'head') {
          const from = Number(cursor ?? 0);
          const to = Math.min(from + limit, lines.length);
          return { body: { lines: lines.slice(from, to), direction: 'head', has_more: to < lines.length, file_size: 1, next_cursor: String(to) } };
        }
        const end = cursor === null ? lines.length : Number(cursor);
        const start = Math.max(0, end - limit);
        return { body: { lines: lines.slice(start, end), direction: 'tail', has_more: start > 0, file_size: 1, next_cursor: String(start) } };
      })
      .start();
  });
  after(() => api.stop());

  it('lists jobs as a table, and as the API document with --json', async () => {
    const human = await lute(['jobs'], { env: env(api.url) });
    assert.match(human.stdout, /SLUG\s+QUEUE\s+LAST RUN/);
    assert.match(human.stdout, /web-release\s+deploy\s+done #66fb1c2d/);
    const json = await lute(['jobs', '--json'], { env: env(api.url) });
    assert.equal(JSON.parse(json.stdout).jobs[0].slug, 'web-release');
  });

  it('shows a job with its parameters and an example', async () => {
    const res = await lute(['jobs', 'show', 'web-release'], { env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    assert.match(res.stdout, /environment\s+select · required/);
    assert.match(res.stdout, /one of: staging, prod/);
    assert.match(res.stdout, /lute run web-release -p environment=staging/);
  });

  it('passes runs filters through', async () => {
    const res = await lute(['runs', '--job', 'web-release', '--status', 'failed', '--limit', '5', '--offset', '10', '--json'], { env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    assert.deepEqual(JSON.parse(res.stdout).echo, { job: 'web-release', status: 'failed', limit: '5', offset: '10' });
  });

  it('prints a whole log across pages', async () => {
    const res = await lute(['logs', '66fb1c2d', '--json'], { env: env(api.url) });
    assert.equal(res.code, 0, res.stderr);
    assert.deepEqual(JSON.parse(res.stdout).lines, lines);
  });

  it('prints the last N lines with --tail, paging back when N is large', async () => {
    const small = await lute(['logs', '66fb1c2d', '--tail', '3'], { env: env(api.url) });
    assert.deepEqual(small.stdout.trim().split('\n'), ['line 1198', 'line 1199', 'line 1200']);
    const big = await lute(['logs', '66fb1c2d', '--tail', '700', '--json'], { env: env(api.url) });
    assert.deepEqual(JSON.parse(big.stdout).lines, lines.slice(-700));
  });

  it('reports the server version without a key', async () => {
    const res = await lute(['version', '--json'], { env: { LUTE_URL: api.url } });
    assert.equal(res.code, 0, res.stderr);
    const v = JSON.parse(res.stdout);
    assert.equal(v.server.version, '0.9.0');
    assert.equal(v.api_level, 1);
  });

  it('warns when the server speaks another API level', async () => {
    const newer = await new FakeLute().on('GET', '/version', () => ({ body: { version: '2.0.0', api_level: 2 } })).start();
    try {
      const res = await lute(['version'], { env: { LUTE_URL: newer.url } });
      assert.equal(res.code, 0);
      assert.match(res.stderr, /API level 1 and the server level 2/);
    } finally {
      await newer.stop();
    }
  });
});
