import { useFlagsStatus, useUnleashClient } from '@unleash/proxy-client-react'
import { useQuery } from '@tanstack/react-query'
import { gameRegistry } from '@/features/lobby/api'
import { keys } from '@/lib/queryClient'

/**
 * useEnabledGames returns the games list filtered by client-side flag state.
 * The server (game-server) already does the authoritative filtering on
 * `GET /api/v1/games`; this hook layers the same check so the UI reflects
 * flag flips instantly in the browser instead of waiting for the next
 * queryClient refetch + server SDK refresh (~15s window).
 *
 * Policy details:
 *
 *   - Before `flagsReady` is true (cold start), filtering is skipped. The
 *     SDK's `isEnabled` returns false for every flag during this window,
 *     which would hide all games — not what we want.
 *   - Once flags are ready, a game with no matching flag is kept (defaults
 *     to enabled). This matches the server default-true behavior.
 *
 * Returns `{ games, isLoading, isError }`. `games` is always an array so
 * callers don't have to deal with `undefined`.
 */
export function useEnabledGames() {
  const unleash = useUnleashClient()
  const { flagsReady } = useFlagsStatus()
  const query = useQuery({
    queryKey: keys.games(),
    queryFn: gameRegistry.list,
  })

  const allGames = query.data ?? []
  const games = !flagsReady
    ? allGames
    : allGames.filter(g => {
        const flagName = `game-${g.id}-enabled`
        // SDK has no "default value" param — so we detect "flag unknown"
        // vs "flag explicitly OFF" via getAllToggles. Unknown → keep.
        const known = unleash.getAllToggles().some(t => t.name === flagName)
        if (!known) return true
        return unleash.isEnabled(flagName)
      })

  return {
    games,
    isLoading: query.isPending,
    isError: query.isError,
  }
}
