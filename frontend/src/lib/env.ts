/**
 * Centralized, typed access to environment variables.
 *
 * All `import.meta.env` reads should go through this module so they are
 * auditable, auto-completable, and have a single source of truth.
 */

/** True in Vite dev server, Vitest, or Docker test mode. */
export const isDev =
  import.meta.env.DEV || import.meta.env.VITE_TEST_MODE === 'true'

/** True only in production builds (no test mode). */
export const isProd = import.meta.env.PROD && !isDev

/** True when running inside Docker with TEST_MODE=true (Playwright e2e). */
export const isTestMode = import.meta.env.VITE_TEST_MODE === 'true'
