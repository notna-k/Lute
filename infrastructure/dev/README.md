# Development Docker Compose Setup

This directory contains Docker Compose configuration for local development.

## Services

- **api**: Go backend (REST, WebSocket, gRPC) with the React UI embedded and served on the same HTTP port; persists **SQLite** at `SQLITE_PATH` (default `/data/lute.db` in Compose via volume `sqlite_data`) for domain data **and** the job queue/state

## Quick Start

1. **Create `.env` file** in this directory (`infrastructure/dev/.env`):
   ```bash
   cd infrastructure/dev
   touch .env
   ```

2. **Configure Firebase** (Required for authentication):
   
   Add these required variables to your `.env` file:
   
   ```bash
   # Firebase Backend Configuration
   FIREBASE_PROJECT_ID=your-firebase-project-id
   FIREBASE_CREDENTIALS_JSON='{"type":"service_account",...}'
   
   # Firebase Frontend Configuration (baked into the UI at API image build time)
   VITE_FIREBASE_API_KEY=your-api-key
   VITE_FIREBASE_AUTH_DOMAIN=your-project-id.firebaseapp.com
   VITE_FIREBASE_PROJECT_ID=your-firebase-project-id
   VITE_FIREBASE_STORAGE_BUCKET=your-project-id.appspot.com
   VITE_FIREBASE_MESSAGING_SENDER_ID=your-sender-id
   VITE_FIREBASE_APP_ID=your-app-id
   ```
   
   **Where to get these values:**
   - Go to [Firebase Console](https://console.firebase.google.com/)
   - For frontend config: Project Settings > Your apps > Firebase SDK snippet
   - For backend config: Project Settings > Service Accounts > Generate new private key
   
   See the full `.env` template at the end of this README.

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
- **gRPC**: Port configuration
- **WebSocket**: Buffer sizes and origin checking
- **Firebase**: All `VITE_FIREBASE_*` variables (required for the embedded UI build inside the `api` image)
- **Ports**: All port mappings for services

### Important: Vite Environment Variables

Vite environment variables (prefixed with `VITE_`) must be available at **build time**, not runtime. Compose passes them as build arguments to the **`api`** image, which runs `npm run build` for the UI and embeds the result in the Go binary. `VITE_API_URL` is left empty so the browser uses the same origin as the API.

**Required Firebase variables:**
- `VITE_FIREBASE_API_KEY`
- `VITE_FIREBASE_AUTH_DOMAIN`
- `VITE_FIREBASE_PROJECT_ID`
- `VITE_FIREBASE_STORAGE_BUCKET`
- `VITE_FIREBASE_MESSAGING_SENDER_ID`
- `VITE_FIREBASE_APP_ID`

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