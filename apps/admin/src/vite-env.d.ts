/// <reference types="vite/client" />

interface ImportMetaEnv {
  /** Префикс API бэкенда (apps/admin/.env.example); фолбэк в коде — '/api'. */
  readonly VITE_API_PREFIX?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
