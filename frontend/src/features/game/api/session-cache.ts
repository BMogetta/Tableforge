import { GameSessionDTO } from '@/lib/api-generated'

/**
 * Unified cache shape for session queries written to the React Query cache
 * under keys.session(id).
 *
 * Both the HTTP response (SessionResponse) and WS-patched updates
 * (game_over, move_applied) share this structure. Using a single type
 * prevents shape mismatches between setQueryData calls and the
 * refetchInterval / useEffect reads in useGameSession.
 */
export interface SessionCache {
  session: GameSessionDTO
  state: unknown
  result?: {
    winner_id?: string | null
    is_draw?: boolean
  } | null
}
