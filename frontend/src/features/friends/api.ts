import { request } from '@/lib/api'

// --- Types -------------------------------------------------------------------

export interface FriendshipView {
  friend_id: string
  friend_username: string
  friend_avatar_url?: string
  status: 'pending' | 'accepted' | 'blocked'
  created_at: string
}

export interface DMConversation {
  other_player_id: string
  other_username: string
  other_avatar_url?: string
  last_message: string
  last_message_at: string
  unread_count: number
}

// --- Friends -----------------------------------------------------------------

export const friends = {
  list: (playerId: string) =>
    request<FriendshipView[]>(`/players/${playerId}/friends`),

  pending: (playerId: string) =>
    request<FriendshipView[]>(`/players/${playerId}/friends/pending`),

  sendRequest: (playerId: string, targetId: string) =>
    request<unknown>(`/players/${playerId}/friends/${targetId}`, { method: 'POST' }),

  acceptRequest: (playerId: string, requesterId: string) =>
    request<unknown>(`/players/${playerId}/friends/${requesterId}/accept`, { method: 'PUT' }),

  declineRequest: (playerId: string, requesterId: string) =>
    request<void>(`/players/${playerId}/friends/${requesterId}/decline`, { method: 'DELETE' }),

  remove: (playerId: string, friendId: string) =>
    request<void>(`/players/${playerId}/friends/${friendId}`, { method: 'DELETE' }),
}

// --- DM conversations --------------------------------------------------------

export const dmConversations = {
  list: (playerId: string) =>
    request<DMConversation[]>(`/players/${playerId}/dm/conversations`),
}

// Re-export dm from room/api for convenience — DM send/history/markRead stay there.
export { dm } from '@/features/room/api'

// --- Presence ----------------------------------------------------------------

export const presence = {
  check: (ids: string[]) =>
    request<Record<string, boolean>>(`/presence?ids=${ids.join(',')}`),
}
