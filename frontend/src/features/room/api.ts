import { request, validatedRequest } from '@/lib/api'
import type {
  CreateRoomRequest,
  JoinRoomRequest,
  UpdateRoomSettingRequest,
  AddBotRequest,
  SendRoomMessageRequest,
  SendDmRequest,
  ReportDmRequest,
} from '@/lib/schema-generated.zod'
import {
  createRoomResponseSchema,
  joinRoomResponseSchema,
  getRoomResponseSchema,
  gameSessionSchema,
  botProfileSchema,
  addBotResponseSchema,
  sendRoomMessageResponseSchema,
  getRoomMessagesResponseSchema,
  sendDmResponseSchema,
  getDmHistoryResponseSchema,
  getDmUnreadCountResponseSchema,
  getMutesResponseSchema,
} from '@/lib/schema-generated.zod'
import { z } from 'zod'

// --- Rooms -------------------------------------------------------------------

export const rooms = {
  list: () => validatedRequest(z.array(getRoomResponseSchema), '/rooms'),
  get: (id: string) => validatedRequest(getRoomResponseSchema, `/rooms/${id}`),
  create: (gameId: string, _playerId: string, turnTimeoutSecs?: number) => {
    const body: CreateRoomRequest = {
      game_id: gameId,
      ...(turnTimeoutSecs != null && { turn_timeout_secs: turnTimeoutSecs }),
    }
    return validatedRequest(createRoomResponseSchema, '/rooms', {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  join: (code: string) => {
    const body: JoinRoomRequest = { code }
    return validatedRequest(joinRoomResponseSchema, '/rooms/join', {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  leave: (roomId: string) => {
    return request<void>(`/rooms/${roomId}/leave`, {
      method: 'POST',
      body: JSON.stringify({}),
    })
  },
  start: (roomId: string) => {
    return validatedRequest(gameSessionSchema, `/rooms/${roomId}/start`, {
      method: 'POST',
      body: JSON.stringify({}),
    })
  },
  updateSetting: (roomId: string, key: string, value: string) => {
    const body: UpdateRoomSettingRequest = { value }
    return request<void>(`/rooms/${roomId}/settings/${encodeURIComponent(key)}`, {
      method: 'PUT',
      body: JSON.stringify(body),
    })
  },
  messages: (roomId: string) =>
    validatedRequest(getRoomMessagesResponseSchema, `/rooms/${roomId}/messages`),
  sendMessage: (roomId: string, playerId: string, content: string) => {
    const body: SendRoomMessageRequest = { content }
    return validatedRequest(sendRoomMessageResponseSchema, `/rooms/${roomId}/messages`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, ...body }),
    })
  },
}

// --- Bots --------------------------------------------------------------------

export const bots = {
  profiles: () => validatedRequest(z.array(botProfileSchema), '/bots/profiles'),
  add: (roomId: string, _playerId: string, profile: string) => {
    const body: AddBotRequest = { profile: profile as AddBotRequest['profile'] }
    return validatedRequest(addBotResponseSchema, `/rooms/${roomId}/bots`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  remove: (roomId: string, _playerId: string, botId: string) => {
    return request<void>(`/rooms/${roomId}/bots/${botId}`, {
      method: 'DELETE',
      body: JSON.stringify({}),
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
    validatedRequest(getMutesResponseSchema, `/players/${playerId}/mutes`),
}

// --- Direct messages ---------------------------------------------------------

export const dm = {
  send: (playerId: string, receiverId: string, content: string) => {
    const body: SendDmRequest = { content }
    return validatedRequest(sendDmResponseSchema, `/players/${receiverId}/dm`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, ...body }),
    })
  },
  history: (playerA: string, playerB: string) =>
    validatedRequest(getDmHistoryResponseSchema, `/players/${playerA}/dm/${playerB}`),
  unreadCount: (playerId: string) =>
    validatedRequest(getDmUnreadCountResponseSchema, `/players/${playerId}/dm/unread`),
  markRead: (callerId: string, messageId: string) =>
    request<void>(`/dm/${messageId}/read`, {
      method: 'POST',
      body: JSON.stringify({ player_id: callerId }),
    }),
  report: (callerId: string, playerA: string, playerB: string, messageId: string) => {
    const body: ReportDmRequest = { message_id: messageId }
    return request<void>(`/players/${playerA}/dm/${playerB}/report`, {
      method: 'POST',
      body: JSON.stringify({ player_id: callerId, ...body }),
    })
  },
}
