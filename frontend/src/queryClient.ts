import { QueryClient } from '@tanstack/react-query'

export const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      // Retry once on failure — enough for transient errors, not too aggressive.
      retry: 1,
      // Keep data fresh for 30s before refetching in background.
      staleTime: 30_000,
      // Refetch when window regains focus — keeps lobby room list current.
      refetchOnWindowFocus: true,
    },
    mutations: {
      retry: 0,
    },
  },
})

// Query key factory — centralizes all keys to avoid typos and simplify invalidation.
export const keys = {
  rooms:      ()                  => ['rooms']          as const,
  room:       (id: string)        => ['rooms', id]      as const,
  roomMessages: (roomId: string) => ['rooms', roomId, 'messages'] as const,
  games:      ()                  => ['games']          as const,
  gameConfig: (id: string)        => ['games', id, 'config'] as const,
  leaderboard: (gameId?: string)  => ['leaderboard', gameId ?? 'all'] as const,
  session:    (id: string)        => ['sessions', id]   as const,
  player:     (id: string)        => ['players', id]    as const,
  adminEmails: ()                 => ['admin', 'emails'] as const,
  adminPlayers: ()                => ['admin', 'players'] as const,
  notifications: (playerId: string) => ['notifications', playerId] as const,
  dmUnread:      (playerId: string) => ['dm', playerId, 'unread']  as const,
  mutes: (playerId: string) => ['mutes', playerId] as const,
}