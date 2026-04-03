import { request, validatedRequest } from '@/lib/api'
import type { RoomMessage, PlayerMute, DirectMessage } from '@/lib/api'
import type {
  CreateRoomRequest,
  JoinRoomRequest,
  LeaveRoomRequest,
  StartGameRequest,
  UpdateRoomSettingRequest,
  AddBotRequest,
  RemoveBotRequest,
} from '@/lib/schema-generated.zod'
import {
  createRoomResponseSchema,
  joinRoomResponseSchema,
  getRoomResponseSchema,
  gameSessionSchema,
  botProfileSchema,
  addBotResponseSchema,
} from '@/lib/schema-generated.zod'
import { z } from 'zod'

// --- Rooms -------------------------------------------------------------------

export const rooms = {
  list: () => validatedRequest(z.array(getRoomResponseSchema), '/rooms'),
  get: (id: string) => validatedRequest(getRoomResponseSchema, `/rooms/${id}`),
  create: (gameId: string, playerId: string, turnTimeoutSecs?: number) => {
    const body: CreateRoomRequest = {
      game_id: gameId,
      player_id: playerId,
      ...(turnTimeoutSecs != null && { turn_timeout_secs: turnTimeoutSecs }),
    }
    return validatedRequest(createRoomResponseSchema, '/rooms', {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  join: (code: string, playerId: string) => {
    const body: JoinRoomRequest = { code, player_id: playerId }
    return validatedRequest(joinRoomResponseSchema, '/rooms/join', {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  leave: (roomId: string, playerId: string) => {
    const body: LeaveRoomRequest = { player_id: playerId }
    return request<void>(`/rooms/${roomId}/leave`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  start: (roomId: string, playerId: string) => {
    const body: StartGameRequest = { player_id: playerId }
    return validatedRequest(gameSessionSchema, `/rooms/${roomId}/start`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  updateSetting: (roomId: string, playerId: string, key: string, value: string) => {
    const body: UpdateRoomSettingRequest = { player_id: playerId, value }
    return request<void>(`/rooms/${roomId}/settings/${encodeURIComponent(key)}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    })
  },
  messages: (roomId: string) => request<RoomMessage[]>(`/rooms/${roomId}/messages`),
  sendMessage: (roomId: string, playerId: string, content: string) =>
    request<RoomMessage>(`/rooms/${roomId}/messages`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, content }),
    }),
}

// --- Bots --------------------------------------------------------------------

export const bots = {
  profiles: () => validatedRequest(z.array(botProfileSchema), '/bots/profiles'),
  add: (roomId: string, playerId: string, profile: string) => {
    const body: AddBotRequest = { player_id: playerId, profile: profile as AddBotRequest['profile'] }
    return validatedRequest(addBotResponseSchema, `/rooms/${roomId}/bots`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  remove: (roomId: string, playerId: string, botId: string) => {
    const body: RemoveBotRequest = { player_id: playerId }
    return request<void>(`/rooms/${roomId}/bots/${botId}`, {
      method: 'DELETE',
      body: JSON.stringify(body),
    })
  },
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
  list: (playerId: string) =>
    request<PlayerMute[]>(`/players/${playerId}/mutes`),
}

// --- Direct messages ---------------------------------------------------------

export const dm = {
  send: (playerId: string, receiverId: string, content: string) =>
    request<DirectMessage>(`/players/${receiverId}/dm`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, content }),
    }),
  history: (playerA: string, playerB: string) =>
    request<DirectMessage[]>(`/players/${playerA}/dm/${playerB}`),
  unreadCount: (playerId: string) =>
    request<{ count: number }>(`/players/${playerId}/dm/unread`),
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
