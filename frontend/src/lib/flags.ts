/**
 * Unleash client config for the frontend.
 *
 * The service-side (Go) SDK hits `http://unleash:4242/api` on the internal
 * docker network. The frontend SDK goes through Traefik to the public
 * host — same Unleash, different path (`/api/frontend` is a read-only view).
 *
 * Values are overridable via Vite env vars so a prod build can point at a
 * different Unleash host without rebuilding the image (future: `VITE_UNLEASH_URL`
 * baked at build time for the k3s deploy).
 */

export const flagsConfig = {
  url: import.meta.env.VITE_UNLEASH_URL ?? 'http://unleash.localhost/api/frontend',
  clientKey:
    import.meta.env.VITE_UNLEASH_CLIENT_KEY ?? '*:*.unleash-insecure-api-token',
  appName: 'frontend',
  environment: import.meta.env.VITE_UNLEASH_ENV ?? 'development',
  refreshInterval: 15,
  disableMetrics: false,
}

/**
 * Canonical flag names. Importing these keeps typos from silently disabling
 * gates (a missing flag defaults to its fallback, not an error).
 */
export const Flags = {
  MaintenanceMode: 'maintenance-mode',
  RankedMatchmakingEnabled: 'ranked-matchmaking-enabled',
  ChatEnabled: 'chat-enabled',
  AchievementsEnabled: 'achievements-enabled',
  GameTicTacToeEnabled: 'game-tictactoe-enabled',
  GameRootAccessEnabled: 'game-rootaccess-enabled',
  DevtoolsForAdmins: 'devtools-for-admins',
} as const
