import { useQuery, useQueryClient } from '@tanstack/react-query'
import { useFlagsStatus, useUnleashClient } from '@unleash/proxy-client-react'
import { useEffect } from 'react'
import { gameRegistry } from '@/features/lobby/api'
import { keys } from '@/lib/queryClient'

/**
 * useEnabledGames returns the games list filtered by client-side flag state.
 * The server (game-server) already does the authoritative filtering on
 * `GET /api/v1/games`; this hook layers the same check so the UI reflects
 * flag flips instantly in the browser.
 *
 * Freshness strategy:
 *
 *   - The games query uses a short staleTime so React Query refetches on
 *     any explicit invalidate or focus change.
 *   - We subscribe to the Unleash client's `update` event and invalidate
 *     the games query whenever flags change. This keeps the server-returned
 *     list aligned with the latest flag state without forcing the user to
 *     reload.
 *   - The client-side filter is kept as belt-and-suspenders so the UI
 *     reflects a flag flip even in the tiny window before the refetch
 *     resolves.
 *
 * Cold-start: before `flagsReady` is true, the SDK's `isEnabled` returns
 * false for every flag. Filtering is skipped in that window so we don't
 * flash an empty lobby.
 *
 * Returns `{ games, isLoading, isError }`. `games` is always an array so
 * callers don't have to deal with `undefined`.
 */
export function useEnabledGames() {
  const unleash = useUnleashClient()
  const { flagsReady } = useFlagsStatus()
  const qc = useQueryClient()

  const query = useQuery({
    queryKey: keys.games(),
    queryFn: gameRegistry.list,
    staleTime: 0,
  })

  // Invalidate the games query whenever flag state changes server-side.
  // TinyEmitter's `on` returns the emitter; we clean up via `off` on unmount.
  useEffect(() => {
    const handler = () => qc.invalidateQueries({ queryKey: keys.games() })
    unleash.on('update', handler)
    return () => {
      unleash.off('update', handler)
    }
  }, [unleash, qc])

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
