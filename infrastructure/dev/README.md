# Development Docker Compose Setup

This directory contains Docker Compose configuration for local development.

## Services

- **api**: Go backend (REST, WebSocket, gRPC) with the React UI embedded and served on the same HTTP port; persists **SQLite** at `SQLITE_PATH` (default `/data/lute.db` in Compose via volume `sqlite_data`) for domain data **and** the job queue/state

## Quick Start

1. **Create `.env` file** in this directory (`infrastructure/dev/.env`):
   ```bash
   cd infrastructure/dev
   cp .env.example .env
   ```

2. **Configure authentication** (Required):

   The API issues its own JWTs and seeds a bootstrap admin user on first startup. Set these
   in your `.env` file:

   ```bash
   # JWT signing key. MUST be >= 32 bytes.
   #   openssl rand -base64 48
   JWT_SECRET=replace-me-with-a-long-random-string-at-least-32-bytes

   # Bootstrap admin, seeded on first startup if no user with this email exists.
   ADMIN_EMAIL=admin@example.com
   ADMIN_PASSWORD=change-me-on-first-login
   ```

   Optional auth tuning (`JWT_ISSUER`, `ACCESS_TOKEN_TTL`, `REFRESH_TOKEN_TTL`,
   `AUTH_COOKIE_SECURE`) is documented in `.env.example`. `AUTH_COOKIE_SECURE` is `false`
   for local HTTP and MUST be `true` in production.

3. **Start all services** (from project root):
   ```bash
   make dev-up
   
   # Or manually:
   cd infrastructure/dev
   docker compose up -d
   ```

4. **View logs**:
   ```bash
   make dev-logs
   
   # Or manually:
   cd infrastructure/dev
   docker compose logs -f
   ```

5. **Stop services**:
   ```bash
   make dev-down
   
   # Stop and remove volumes (clean slate)
   make dev-clean
   ```

## Access Points

- **App (UI + API)**: http://localhost:8080
- **API Health**: http://localhost:8080/api/health
- **gRPC**: localhost:50051

## Environment Variables

All environment variables are configured in the `.env` file. Copy `.env.example` to `.env` and modify as needed:

- **SQLite**: `SQLITE_PATH` (default in Compose: `/data/lute.db` inside the API container; backed by the `sqlite_data` volume)
- **API**: Server ports, Gin mode, SQLite path and busy timeout
- **Auth**: `JWT_SECRET`, `ADMIN_EMAIL`, `ADMIN_PASSWORD`, plus optional token TTLs
- **gRPC**: Port configuration
- **WebSocket**: Buffer sizes and origin checking
- **Ports**: All port mappings for services

### Important: Vite Environment Variables

Vite environment variables (prefixed with `VITE_`) must be available at **build time**, not runtime. Compose passes them as build arguments to the **`api`** image, which runs `npm run build` for the UI and embeds the result in the Go binary. `VITE_API_URL` is left empty so the browser uses the same origin as the API.

Authentication needs no build-time variables — the UI signs in against the API's `/api/v1/auth`
endpoints at runtime.

See `.env.example` for all available variables and their defaults.

## Rebuilding Services

```bash
# Rebuild API (includes UI bundle)
docker compose build api

# Rebuild and restart
docker compose up -d --build api
```

## Volumes

- `sqlite_data`: Persistent SQLite directory (`/data` in the API container)