/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** When unset or empty, API calls use the current origin (embedded single-binary deploy). */
  readonly VITE_API_URL?: string

  readonly VITE_FIREBASE_API_KEY: string
  readonly VITE_FIREBASE_AUTH_DOMAIN: string
  readonly VITE_FIREBASE_PROJECT_ID: string
  readonly VITE_FIREBASE_STORAGE_BUCKET: string
  readonly VITE_FIREBASE_MESSAGING_SENDER_ID: string
  readonly VITE_FIREBASE_APP_ID: string
  /** Metrics graph refetch interval in seconds (e.g. "5" for 5s). Optional. */
  readonly VITE_METRICS_UPDATE_INTERVAL_SECONDS?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

