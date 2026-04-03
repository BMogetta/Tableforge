import { request, validatedRequest } from '@/lib/api'
import type { QueuePosition } from '@/lib/api'
import { gameInfoSchema, getLeaderboardResponseSchema } from '@/lib/schema-generated.zod'
import { z } from 'zod'

// --- Leaderboard -------------------------------------------------------------

export const leaderboard = {
  get: async (gameId: string, limit = 20) => {
    const params = new URLSearchParams({ limit: String(limit) })
    const res = await validatedRequest(getLeaderboardResponseSchema, `/ratings/${gameId}/leaderboard?${params}`)
    return res.entries
  },
}

// --- Game registry -----------------------------------------------------------

export const gameRegistry = {
  list: () => validatedRequest(z.array(gameInfoSchema), '/games'),
}

// --- Queue -------------------------------------------------------------------

export const queue = {
  join: (playerId: string) =>
    request<QueuePosition>('/queue', {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  leave: (playerId: string) =>
    request<void>('/queue', {
      method: 'DELETE',
      body: JSON.stringify({ player_id: playerId }),
    }),
  accept: (playerId: string, matchId: string) =>
    request<void>('/queue/accept', {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, match_id: matchId }),
    }),
  decline: (playerId: string, matchId: string) =>
    request<void>('/queue/decline', {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, match_id: matchId }),
    }),
}
