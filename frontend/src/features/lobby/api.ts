import { request, validatedRequest } from '@/lib/api'
import type { AcceptMatchRequest, DeclineMatchRequest } from '@/lib/schema-generated.zod'
import {
  gameInfoSchema,
  getLeaderboardResponseSchema,
  queuePositionSchema,
} from '@/lib/schema-generated.zod'
import { z } from 'zod'

// --- Leaderboard -------------------------------------------------------------

export const leaderboard = {
  get: async (gameId: string, limit = 20, includeBots = false) => {
    const params = new URLSearchParams({ limit: String(limit) })
    if (includeBots) params.set('include_bots', 'true')
    const res = await validatedRequest(
      getLeaderboardResponseSchema,
      `/ratings/${gameId}/leaderboard?${params}`,
    )
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
    validatedRequest(queuePositionSchema, '/queue', {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  leave: (playerId: string) =>
    request<void>('/queue', {
      method: 'DELETE',
      body: JSON.stringify({ player_id: playerId }),
    }),
  accept: (_playerId: string, matchId: string) => {
    const body: AcceptMatchRequest = { match_id: matchId }
    return request<void>('/queue/accept', {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  decline: (_playerId: string, matchId: string) => {
    const body: DeclineMatchRequest = { match_id: matchId }
    return request<void>('/queue/decline', {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
}
