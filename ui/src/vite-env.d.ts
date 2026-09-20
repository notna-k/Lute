/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** When unset or empty, API calls use the current origin (embedded single-binary deploy). */
  readonly VITE_API_URL?: string

  /** Metrics graph refetch interval in seconds (e.g. "5" for 5s). Optional. */
  readonly VITE_METRICS_UPDATE_INTERVAL_SECONDS?: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}
