/// <reference types="vite/client" />

// Build-time settings read from .env files or the environment.
interface ImportMetaEnv {
  /** Where the API lives, e.g. https://api.example.com. Empty means the same origin as the UI. */
  readonly VITE_API_BASE_URL?: string
}
