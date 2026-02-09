# Development Docker Compose Setup

This directory contains Docker Compose configuration for local development.

## Services

- **mongodb**: MongoDB 7.0 database
- **api**: Go backend API server
- **ui**: React frontend application

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
   
   # Firebase Frontend Configuration (for UI build)
   VITE_FIREBASE_API_KEY=your-api-key
   VITE_FIREBASE_AUTH_DOMAIN=your-project-id.firebaseapp.com
   VITE_FIREBASE_PROJECT_ID=your-firebase-project-id
   VITE_FIREBASE_STORAGE_BUCKET=your-project-id.appspot.com
   VITE_FIREBASE_MESSAGING_SENDER_ID=your-sender-id
   VITE_FIREBASE_APP_ID=your-app-id
   VITE_API_URL=http://localhost:8080
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

- **UI**: http://localhost:3000
- **API**: http://localhost:8080
- **API Health**: http://localhost:8080/api/health
- **MongoDB**: localhost:27017
- **gRPC**: localhost:50051

## MongoDB Credentials

- Username: `admin`
- Password: `admin123`
- Database: `lute`

### Connecting with MongoDB Compass

Use this connection string in MongoDB Compass:

```
mongodb://admin:admin123@localhost:27017/lute?authSource=admin
```

Or connect manually:
- **Host**: `localhost`
- **Port**: `27017`
- **Authentication**: Username/Password
- **Username**: `admin`
- **Password**: `admin123`
- **Authentication Database**: `admin`
- **Default Database**: `lute`

## Environment Variables

All environment variables are configured in the `.env` file. Copy `.env.example` to `.env` and modify as needed:

- **MongoDB**: Credentials, database name, connection URI
- **API**: Server ports, Gin mode, MongoDB connection
- **gRPC**: Port configuration
- **WebSocket**: Buffer sizes and origin checking
- **UI**: API URL for frontend
- **Firebase**: All `VITE_FIREBASE_*` variables (required for UI build)
- **Ports**: All port mappings for services

### Important: Vite Environment Variables

Vite environment variables (prefixed with `VITE_`) must be available at **build time**, not runtime. They are passed as build arguments to the Docker build process and embedded into the JavaScript bundle during `npm run build`.

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
# Rebuild specific service
docker compose build api
docker compose build ui

# Rebuild and restart
docker compose up -d --build api
```

## Volumes

- `mongodb_data`: Persistent MongoDB data storage
- `mongodb_config`: MongoDB configuration storage