# lute

The command line for [Lute](https://github.com/notna-k/Lute), a self-hosted CI server: find a job,
run it with typed parameters, follow its log and act on the result, from a terminal, a script, CI or
an LLM agent.

```bash
npm i -g @notna-k/lute-cli
lute login                     # the panel URL and an API key from Settings → API keys
lute jobs
lute run web-release -p environment=staging --follow
```

In CI, set `LUTE_URL` and `LUTE_API_KEY` (a service key) and skip the login. `--json` and the exit
codes (0 ok, 1 run failed, 2 usage, 3 auth, 4 not found, 5 unreachable) are a stable contract.

Docs: https://notna-k.github.io/Lute/docs/cli/

Requires Node.js 22.18 or newer. The API key is kept in the system keychain and never printed.

## Developing

```bash
npm ci --ignore-scripts
npm test              # node:test, the real lute process against a stand-in API
npm run typecheck
npm run gen:api       # after api/openapi.yaml changes
npm run build         # dist/main.js
```
