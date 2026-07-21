# Lute UI — Worker Management Dashboard

A React application for operating workers, queues, and jobs, built with Vite, TypeScript, and the
Lute API's JWT authentication.

## Features

- 🔐 Email/password authentication against the Lute API (JWT access + refresh tokens)
- 📊 Dashboard with fleet statistics and metric snapshots
- 🖥️ Worker list and detail views, including label editing
- ⚙️ Job enqueueing with optional label selectors
- 🔑 API key management
- 🎨 Modern UI with Tailwind CSS
- ⚡ Fast development with Vite

## Getting Started

### Prerequisites

- Node.js **25.x** (see `engines` in `package.json`, `.nvmrc`) and npm
- A running Lute API (see `infrastructure/dev/README.md`)

### Installation

1. Install dependencies:
```bash
npm install
```

2. Configure the API endpoint:

   Create a `.env` file in the `ui` directory if the API is not served from the same origin:

   ```bash
   # Leave empty to use the same origin as the page (the default when the UI is
   # embedded in the API binary).
   VITE_API_URL=http://localhost:8080
   ```

   No auth provider configuration is needed — sign-in goes to the API's `/api/v1/auth` endpoints.
   The bootstrap admin account is seeded by the API from `ADMIN_EMAIL` / `ADMIN_PASSWORD`.

### Development

Start the development server:

```bash
npm run dev
```

The app will be available at `http://localhost:3000` (see `vite.config.ts`).

### Build

Build for production:

```bash
npm run build
```

The production build will be in the `dist` directory. `make api-build` embeds it into the API binary.

### Preview

Preview the production build:

```bash
npm run preview
```

## Project Structure

```
ui/
├── src/
│   ├── components/       # Reusable React components
│   │   └── layout/       # App shell, navbar, etc.
│   ├── contexts/         # React contexts
│   │   ├── AuthContext.tsx
│   │   └── ThemeContext.tsx
│   ├── features/         # Feature-scoped components (jobs, workers, ...)
│   ├── hooks/            # Shared hooks
│   ├── lib/              # Low-level helpers
│   ├── pages/            # Page components
│   │   ├── Dashboard.tsx
│   │   ├── Login.tsx
│   │   ├── Workers.tsx
│   │   ├── WorkerDetail.tsx
│   │   ├── Executions.tsx
│   │   ├── ExecutionDetail.tsx
│   │   └── Settings.tsx
│   ├── services/         # API client and service functions
│   │   ├── api.ts
│   │   └── authService.ts
│   ├── types/            # TypeScript type definitions
│   ├── utils/            # Utilities
│   ├── App.tsx           # Main App component
│   ├── main.tsx          # Application entry point
│   └── index.css         # Global styles
├── index.html            # HTML template
├── vite.config.ts        # Vite configuration
├── tsconfig.json         # TypeScript configuration
└── package.json          # Dependencies and scripts
```

## Technologies

- **React 18** - UI library
- **TypeScript** - Type safety
- **Vite** - Build tool and dev server
- **React Router** - Client-side routing
- **TanStack Query** - Server state and caching
- **Tailwind CSS** - Styling
