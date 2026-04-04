import { request } from '@/lib/api'
import type {
  FriendshipView,
  DmConversation,
  PlayerSearchResult,
} from '@/lib/schema-generated.zod'

// Re-export for convenience so consumers don't need to know the source.
export type { FriendshipView, PlayerSearchResult } from '@/lib/schema-generated.zod'
// Preserve existing name casing used by consumers.
export type { DmConversation as DMConversation } from '@/lib/schema-generated.zod'

// --- Player search -----------------------------------------------------------

export const players = {
  search: (username: string) =>
    request<PlayerSearchResult>(`/players/search?username=${encodeURIComponent(username)}`),
}

// --- Friends -----------------------------------------------------------------

export const friends = {
  list: (playerId: string) =>
    request<FriendshipView[]>(`/players/${playerId}/friends`),

  pending: (playerId: string) =>
    request<FriendshipView[]>(`/players/${playerId}/friends/pending`),

  sendRequest: (playerId: string, targetId: string) =>
    request<unknown>(`/players/${playerId}/friends/${targetId}`, { method: 'POST', body: JSON.stringify({}) }),

  acceptRequest: (playerId: string, requesterId: string) =>
    request<unknown>(`/players/${playerId}/friends/${requesterId}/accept`, { method: 'PUT', body: JSON.stringify({}) }),

  declineRequest: (playerId: string, requesterId: string) =>
    request<void>(`/players/${playerId}/friends/${requesterId}/decline`, { method: 'DELETE', body: JSON.stringify({}) }),

  remove: (playerId: string, friendId: string) =>
    request<void>(`/players/${playerId}/friends/${friendId}`, { method: 'DELETE', body: JSON.stringify({}) }),
}

// --- DM conversations --------------------------------------------------------

export const dmConversations = {
  list: (playerId: string) =>
    request<DmConversation[]>(`/players/${playerId}/dm/conversations`),
}

// Re-export dm from room/api for convenience — DM send/history/markRead stay there.
export { dm } from '@/features/room/api'

// --- Blocks ------------------------------------------------------------------

export interface BlockedPlayer {
  id: string
  username: string
  avatar_url?: string
  blocked_at: string
}

export const blocks = {
  block: (playerId: string, targetId: string) =>
    request<unknown>(`/players/${playerId}/block/${targetId}`, { method: 'POST' }),

  unblock: (playerId: string, targetId: string) =>
    request<void>(`/players/${playerId}/block/${targetId}`, { method: 'DELETE' }),
}

// --- Presence ----------------------------------------------------------------

export const presence = {
  check: (ids: string[]) =>
    request<Record<string, boolean>>(`/presence?ids=${ids.join(',')}`),
}
