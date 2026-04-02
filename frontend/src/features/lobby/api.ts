import { request } from '@/lib/api'
import type { Rating, GameInfo, QueuePosition } from '@/lib/api'

// --- Leaderboard -------------------------------------------------------------

export const leaderboard = {
  get: async (gameId: string, limit = 20) => {
    const params = new URLSearchParams({ limit: String(limit) })
    const res = await request<{ entries: Rating[]; total: number }>(`/ratings/${gameId}/leaderboard?${params}`)
    return res.entries
  },
}

// --- Game registry -----------------------------------------------------------

export const gameRegistry = {
  list: () => request<GameInfo[]>('/games'),
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

