import { useEffect } from 'react'
import { useQuery } from '@tanstack/react-query'
import { rooms } from '@/features/room/api'
import { keys } from '@/lib/queryClient'
import type { GameData } from '@/games/registry'
import type { GameSession } from '@/lib/schema-generated.zod'
import { sessions } from '@/lib/api/sessions'

interface UseGameSessionOptions {
  sessionId: string
  onGameOver: (winnerId: string | null, isDraw: boolean) => void
}

interface UseGameSessionReturn {
  session: GameSession | null
  gameData: GameData | null
  roomPlayers: {
    id: string
    username: string
    is_bot: boolean
    bot_profile?: 'easy' | 'medium' | 'hard' | 'aggressive'
  }[]
  loadError: Error | null
}

/**
 * Manages fetching and syncing game session state.
 *
 * Responsibilities:
 * - Fetches the session once on mount. WS events (move_applied, game_over)
 *   keep the cache in sync — no polling needed.
 * - On WS reconnect, useGameSocket invalidates the session query to resync.
 * - Fetches room players (stale forever — they don't change mid-game).
 * - Calls onGameOver when the session's finished_at is set on initial load
 *   (covers the "player returns to an already-finished game" case).
 *
 * @testability
 * Mock `sessions.get` and `rooms.get` from `../../lib/api`.
 * Pass a spy as `onGameOver` to assert it's called with the right arguments.
 */
export function useGameSession({
  sessionId,
  onGameOver,
}: UseGameSessionOptions): UseGameSessionReturn {
  const { data, error: loadError } = useQuery({
    queryKey: keys.session(sessionId),
    queryFn: () => sessions.get(sessionId),
    refetchOnWindowFocus: false,
    staleTime: Infinity,
  })

  const session = data?.session ?? null
  const gameData = (data?.state as GameData) ?? null

  useEffect(() => {
    if (!data?.session?.finished_at) return
    // Guard: if result is absent, the WS game_over event already called
    // onGameOver with the correct winner. Do not overwrite it with null.
    if (!data?.result) return
    onGameOver(data.result.winner_id ?? null, data.result.is_draw ?? false)
  }, [data?.session?.finished_at, data?.result?.winner_id, data?.result, onGameOver])

  const { data: roomData } = useQuery({
    queryKey: keys.room(session?.room_id ?? ''),
    queryFn: () => rooms.get(session!.room_id),
    enabled: !!session?.room_id,
    staleTime: Infinity,
  })

  const roomPlayers =
    roomData?.players.map(p => ({
      id: p.id,
      username: p.username,
      is_bot: p.is_bot,
      bot_profile: p.bot_profile,
    })) ?? []

  return {
    session,
    gameData,
    roomPlayers,
    loadError: loadError as Error | null,
  }
}
