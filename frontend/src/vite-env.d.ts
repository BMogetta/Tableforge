/// <reference types="vite/client" />
/// <reference types="@testing-library/jest-dom" />

interface ImportMetaEnv {
  /** True when running on Vite dev server or in Vitest. */
  readonly DEV: boolean
  /** True when running in production mode. */
  readonly PROD: boolean
  /** "development" | "production" */
  readonly MODE: string
  /** Set to "true" in Docker test builds for Playwright (enables testIds). */
  readonly VITE_TEST_MODE?: string
  /** Build version injected by vite.config.ts from `git describe --tags --dirty`. */
  readonly VITE_APP_VERSION: string
}

interface ImportMeta {
  readonly env: ImportMetaEnv
}

declare module '*.module.css' {
  const classes: Record<string, string>
  export default classes
}
