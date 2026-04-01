import { request } from '@/lib/api'
import type {
  RoomView,
  RoomMessage,
  PlayerMute,
  DirectMessage,
  BotProfile,
  Player,
} from '@/lib/api'
import type { GameSessionDTO } from '@/lib/api-generated'

// --- Rooms -------------------------------------------------------------------

export const rooms = {
  list: () => request<RoomView[]>('/rooms'),
  get: (id: string) => request<RoomView>(`/rooms/${id}`),
  create: (gameId: string, playerId: string, turnTimeoutSecs?: number) =>
    request<RoomView>('/rooms', {
      method: 'POST',
      body: JSON.stringify({
        game_id: gameId,
        player_id: playerId,
        turn_timeout_secs: turnTimeoutSecs,
      }),
    }),
  join: (code: string, playerId: string) =>
    request<RoomView>('/rooms/join', {
      method: 'POST',
      body: JSON.stringify({ code, player_id: playerId }),
    }),
  leave: (roomId: string, playerId: string) =>
    request<void>(`/rooms/${roomId}/leave`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  start: (roomId: string, playerId: string) =>
    request<GameSessionDTO>(`/rooms/${roomId}/start`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  updateSetting: (roomId: string, playerId: string, key: string, value: string) =>
    request<void>(`/rooms/${roomId}/settings/${encodeURIComponent(key)}`, {
      method: 'PUT',
      body: JSON.stringify({ player_id: playerId, value }),
    }),
  messages: (roomId: string) => request<RoomMessage[]>(`/rooms/${roomId}/messages`),
  sendMessage: (roomId: string, playerId: string, content: string) =>
    request<RoomMessage>(`/rooms/${roomId}/messages`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, content }),
    }),
}

// --- Bots --------------------------------------------------------------------

export const bots = {
  profiles: () => request<BotProfile[]>('/bots/profiles'),
  add: (roomId: string, playerId: string, profile: string) =>
    request<Player>(`/rooms/${roomId}/bots`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, profile }),
    }),
  remove: (roomId: string, playerId: string, botId: string) =>
    request<void>(`/rooms/${roomId}/bots/${botId}`, {
      method: 'DELETE',
      body: JSON.stringify({ player_id: playerId }),
    }),
}

// --- Mutes -------------------------------------------------------------------

export const mutes = {
  mute: (playerId: string, mutedId: string) =>
    request<void>(`/players/${mutedId}/mute`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, muted_id: mutedId }),
    }),
  unmute: (playerId: string, mutedId: string) =>
    request<void>(`/players/${playerId}/mute/${mutedId}`, {
      method: 'DELETE',
      body: JSON.stringify({ player_id: playerId }),
    }),
  list: (callerId: string, playerId: string) =>
    request<PlayerMute[]>(`/players/${playerId}/mutes`, {
      method: 'GET',
      body: JSON.stringify({ player_id: callerId }),
    }),
}

// --- Direct messages ---------------------------------------------------------

export const dm = {
  send: (playerId: string, receiverId: string, content: string) =>
    request<DirectMessage>(`/players/${receiverId}/dm`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, content }),
    }),
  history: (callerId: string, playerA: string, playerB: string) =>
    request<DirectMessage[]>(`/players/${playerA}/dm/${playerB}`, {
      method: 'GET',
      body: JSON.stringify({ player_id: callerId }),
    }),
  unreadCount: (playerId: string) =>
    request<{ count: number }>(`/players/${playerId}/dm/unread`, {
      method: 'GET',
      body: JSON.stringify({ player_id: playerId }),
    }),
  markRead: (callerId: string, messageId: string) =>
    request<void>(`/dm/${messageId}/read`, {
      method: 'POST',
      body: JSON.stringify({ player_id: callerId }),
    }),
  report: (callerId: string, playerA: string, playerB: string, messageId: string) =>
    request<void>(`/players/${playerA}/dm/${playerB}/report`, {
      method: 'POST',
      body: JSON.stringify({ player_id: callerId, message_id: messageId }),
    }),
}
