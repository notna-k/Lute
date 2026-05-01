# Lute documentation site

This directory contains the user-facing documentation for Lute's public REST API.

## Layout

- `docs.json` — Mintlify site configuration (navigation, theme, branding).
- `openapi.yaml` — OpenAPI 3.1 spec for the public `/api/public/v1` surface.
  It is the source of truth used by SDK generators and contract tests.
- `introduction.mdx`, `authentication.mdx`, `runs.mdx`, `webhooks.mdx`,
  `idempotency.mdx`, `errors.mdx` — prose guides. These are what users read
  first; `openapi.yaml` is the reference they dip into when they need exact
  field-by-field details.

## Previewing locally

Install the Mintlify CLI once:

```bash
npm install -g mint
```

Then from this directory:

```bash
mint dev
```

The dev server serves the site at `http://localhost:3000`.

## Why Mintlify (and not Swagger UI)

The guides are written as MDX pages with examples and cross-links, which is
better suited to onboarding than a raw Swagger document. The OpenAPI spec is
still published so developers can generate SDKs, run contract tests, or render
an interactive reference with Scalar, Redoc, or Stoplight Elements if they
prefer a different look.
