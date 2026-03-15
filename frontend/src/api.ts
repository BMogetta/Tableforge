// All types mirror the Go store models.

import { tracer } from './telemetry'
import { SpanStatusCode, context, propagation } from '@opentelemetry/api'

// --- Types -------------------------------------------------------------------

export interface Player {
  id: string
  username: string
  role: 'player' | 'manager' | 'owner'
  is_bot: boolean 
  avatar_url?: string
  created_at: string
}

export interface RoomViewPlayer extends Player {
  seat: number
  joined_at: string
}

export interface AllowedEmail {
  email: string
  role: 'player' | 'manager' | 'owner'
  note?: string
  invited_by?: string
  expires_at?: string
  created_at: string
}

export interface Room {
  id: string
  /**
   * The room join code. Empty string ("") for private rooms when returned
   * from GET /rooms (list) — the backend redacts it to prevent discovery.
   * Always populated when returned from GET /rooms/:id (direct fetch by a
   * participant who already knows the room).
   */
  code: string
  game_id: string
  owner_id: string
  status: 'waiting' | 'in_progress' | 'finished'
  max_players: number
  created_at: string
}

export interface RoomMessage {
  message_id: string
  room_id: string
  player_id: string
  content: string
  reported: boolean
  hidden: boolean
  timestamp: string
}

export interface RoomView {
  room: Room
  players: RoomViewPlayer[]
  // Key/value map of all settings for this room.
  // Always present — defaults are inserted when the room is created.
  settings: Record<string, string>
  // Live spectator count. Kept in sync by spectator_joined / spectator_left
  // WS events in Room.tsx; may be absent on initial HTTP fetch.
  spectator_count?: number
}

export interface GameSession {
  id: string
  room_id: string
  game_id: string
  state: unknown
  move_count: number
  started_at: string
  finished_at?: string
}

export interface GameResult {
  winner_id?: string
  status?: string
  ended_by?: string
  is_draw?: boolean
}

export interface Move {
  id: string
  session_id: string
  player_id: string
  payload: unknown
  move_number: number
  applied_at: string
}

export interface PlayerStats {
  player_id: string
  total_games: number
  wins: number
  losses: number
  draws: number
  forfeits: number
}

// Rating mirrors store.Rating on the Go side.
// NOTE: mmr is intentionally absent — it must not reach the frontend.
// If the backend serializes mmr in /leaderboard responses, that is a backend
// bug and should be fixed by excluding it from the JSON response struct.
export interface Rating {
  player_id: string
  username: string
  avatar_url?: string
  display_rating: number
  games_played: number
  win_streak: number
  loss_streak: number
  updated_at: string
}

// Mutes are cross-session and server-side — they survive reconnects.
export interface PlayerMute {
  muter_id: string
  muted_id: string
  created_at: string
}

// DirectMessage mirrors store.DirectMessage on the Go side.
export interface DirectMessage {
  id: string
  sender_id: string
  receiver_id: string
  content: string
  read_at?: string
  reported: boolean
  hidden: boolean
  created_at: string
}

export interface QueuePosition {
  position: number
  estimated_wait_secs: number
}

export interface BotProfile {
  name: string
  iterations: number
  determinizations: number
  exploration_c: number
  aggressiveness: number
  risk_aversion: number
}

export interface SessionEvent {
  id: string
  session_id: string
  type: string
  player_id?: string
  payload?: Record<string, unknown>
  occurred_at: string
}

// --- Notification types ------------------------------------------------------

export type NotificationType = 'friend_request' | 'room_invitation' | 'ban_issued'

export interface NotificationPayloadFriendRequest {
  from_player_id: string
  from_username: string
}

export interface NotificationPayloadRoomInvitation {
  room_id: string
  room_code: string
  from_player_id: string
  from_username: string
}

export interface NotificationPayloadBanIssued {
  reason: 'decline_threshold' | 'moderator'
  expires_at?: string
}

export interface Notification {
  id: string
  player_id: string
  type: NotificationType
  payload: NotificationPayloadFriendRequest | NotificationPayloadRoomInvitation | NotificationPayloadBanIssued
  action_taken?: string
  action_expires_at?: string
  read_at?: string
  created_at: string
}

// --- Lobby settings types ----------------------------------------------------

export interface SettingOption {
  value: string
  label: string
}

/**
 * Describes a single configurable room setting declared by a game.
 * Mirrors engine.LobbySetting on the Go side.
 */
export interface LobbySetting {
  key: string
  label: string
  description?: string
  type: 'select' | 'int'
  default: string
  // Present when type === 'select'
  options?: SettingOption[]
  // Present when type === 'int'
  min?: number
  max?: number
}

/**
 * Public representation of a registered game, including its lobby settings.
 * Mirrors api.gameInfo on the Go side.
 */
export interface GameInfo {
  id: string
  name: string
  min_players: number
  max_players: number
  // Full list of configurable lobby settings for this game,
  // platform defaults merged with any game-specific overrides.
  settings: LobbySetting[]
}

// --- HTTP client -------------------------------------------------------------

const BASE = '/api/v1'

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const method = (init?.method ?? 'GET').toUpperCase()
  const url = BASE + path

  return tracer.startActiveSpan(`${method} ${url}`, async (span) => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(init?.headers as Record<string, string>),
    }
    propagation.inject(context.active(), headers)

    try {
      const res = await fetch(url, {
        ...init,
        headers,
        credentials: 'include',
      })

      span.setAttributes({
        'http.method': method,
        'http.url': url,
        'http.status_code': res.status,
      })

      if (!res.ok) {
        const body = await res.json().catch(() => ({}))
        span.setStatus({ code: SpanStatusCode.ERROR, message: `HTTP ${res.status}` })
        span.end()
        throw new ApiError(res.status, body.error ?? res.statusText)
      }

      span.setStatus({ code: SpanStatusCode.OK })
      span.end()

      if (res.status === 204 || res.headers.get('content-length') === '0') {
        return undefined as T
      }

      return res.json() as Promise<T>
    } catch (err) {
      if (!(err instanceof ApiError)) {
        span.setStatus({
          code: SpanStatusCode.ERROR,
          message: err instanceof Error ? err.message : 'fetch failed',
        })
        span.recordException(err as Error)
        span.end()
      }
      throw err
    }
  })
}

export class ApiError extends Error {
  constructor(public status: number, message: string) {
    super(message)
  }
}

// --- Auth --------------------------------------------------------------------
// We don't use the request() helper for auth routes since they have some
// special handling (e.g. login redirects to GitHub) and we don't want them
// to be traced like normal API calls.

export const auth = {
  me: () => fetch('/auth/me', { credentials: 'include' }).then(async res => {
    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new ApiError(res.status, body.error ?? res.statusText)
    }
    return res.json() as Promise<Player>
  }),
  logout: () => fetch('/auth/logout', { method: 'POST', credentials: 'include' }),
  loginUrl: '/auth/github',
}

// --- Players -----------------------------------------------------------------

export const players = {
  stats: (id: string) => request<PlayerStats>(`/players/${id}/stats`),
  sessions: (id: string) => request<GameSession[]>(`/players/${id}/sessions`),
}

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
    request<GameSession>(`/rooms/${roomId}/start`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  /**
   * Updates a single room setting. Only the room owner can call this while
   * the room is in waiting state. Triggers a `setting_updated` WS event.
   */
  updateSetting: (roomId: string, playerId: string, key: string, value: string) =>
    request<void>(`/rooms/${roomId}/settings/${encodeURIComponent(key)}`, {
      method: 'PUT',
      body: JSON.stringify({ player_id: playerId, value }),
    }),
  messages: (roomId: string) =>
    request<RoomMessage[]>(`/rooms/${roomId}/messages`),
  sendMessage: (roomId: string, playerId: string, content: string) =>
    request<RoomMessage>(`/rooms/${roomId}/messages`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, content }),
    }),
}

// --- Sessions ----------------------------------------------------------------

export const sessions = {
  get: (id: string) =>
    request<{
      session: GameSession
      state: { current_player_id: string; data: unknown }
      result?: GameResult
    }>(`/sessions/${id}`),
  move: (sessionId: string, playerId: string, payload: unknown) =>
    request<{
      session: GameSession
      state: { current_player_id: string; data: unknown }
      is_over: boolean
    }>(`/sessions/${sessionId}/move`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, payload }),
    }),
  surrender: (sessionId: string, playerId: string) =>
    request<{
      session: GameSession
      state: { current_player_id: string; data: unknown }
      is_over: boolean
      result?: GameResult
    }>(`/sessions/${sessionId}/surrender`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  rematch: (sessionId: string, playerId: string) =>
    request<{
      votes: number
      total_players: number
      session?: GameSession
    }>(`/sessions/${sessionId}/rematch`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  pause: (sessionId: string, playerId: string) =>
    request<{ votes: string[]; required: number; all_voted: boolean }>(
      `/sessions/${sessionId}/pause`,
      { method: 'POST', body: JSON.stringify({ player_id: playerId }) },
    ),
  resume: (sessionId: string, playerId: string) =>
    request<{ votes: string[]; required: number; all_voted: boolean }>(
      `/sessions/${sessionId}/resume`,
      { method: 'POST', body: JSON.stringify({ player_id: playerId }) },
    ),
  // Manager only. Broadcasts room_closed to all clients.
  forceClose: (sessionId: string) =>
    request<void>(`/sessions/${sessionId}`, { method: 'DELETE' }),
  events: (id: string) => request<SessionEvent[]>(`/sessions/${id}/events`),
  history: (id: string) => request<Move[]>(`/sessions/${id}/history`),
}

// --- Leaderboard -------------------------------------------------------------

export const leaderboard = {
  /**
   * Returns the top players by display_rating for the given game.
   * game_id is required — ratings are per-game and must not be mixed.
   */
  get: (gameId: string, limit = 20) => {
    const params = new URLSearchParams({ limit: String(limit), game_id: gameId })
    return request<Rating[]>(`/leaderboard?${params}`)
  },
}

// --- Game registry -----------------------------------------------------------

export const gameRegistry = {
  list: () => request<GameInfo[]>('/games'),
}

// --- Admin -------------------------------------------------------------------

export const admin = {
  // Allowed emails
  listEmails: () =>
    request<AllowedEmail[]>('/admin/allowed-emails'),
  addEmail: (email: string, role: 'player' | 'manager' = 'player') =>
    request<AllowedEmail>('/admin/allowed-emails', {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }),
  removeEmail: (email: string) =>
    request<void>(`/admin/allowed-emails/${encodeURIComponent(email)}`, {
      method: 'DELETE',
    }),

  // Players
  listPlayers: () =>
    request<Player[]>('/admin/players'),
  setRole: (playerId: string, role: 'player' | 'manager' | 'owner') =>
    request<void>(`/admin/players/${playerId}/role`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),
}

// --- Mutes -------------------------------------------------------------------

export const mutes = {
  /**
   * Records that playerId has muted mutedId. Idempotent.
   * POST /players/{mutedId}/mute
   */
  mute: (playerId: string, mutedId: string) =>
    request<void>(`/players/${mutedId}/mute`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, muted_id: mutedId }),
    }),

  /**
   * Removes a mute. No-op if the mute does not exist.
   * DELETE /players/{playerId}/mute/{mutedId}
   */
  unmute: (playerId: string, mutedId: string) =>
    request<void>(`/players/${playerId}/mute/${mutedId}`, {
      method: 'DELETE',
      body: JSON.stringify({ player_id: playerId }),
    }),

  /**
   * Returns all players muted by playerId.
   * Managers may pass any playerId; regular players may only pass their own.
   * GET /players/{playerId}/mutes
   */
  list: (callerId: string, playerId: string) =>
    request<PlayerMute[]>(`/players/${playerId}/mutes`, {
      method: 'GET',
      body: JSON.stringify({ player_id: callerId }),
    }),
}

// --- Direct messages ---------------------------------------------------------

export const dm = {
  /**
   * Sends a DM from playerId to receiverId.
   * POST /players/{receiverId}/dm
   */
  send: (playerId: string, receiverId: string, content: string) =>
    request<DirectMessage>(`/players/${receiverId}/dm`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, content }),
    }),

  /**
   * Returns the full conversation history between two players.
   * Caller must be one of the two participants or hold owner role.
   * GET /players/{playerID}/dm/{otherPlayerID}
   */
  history: (callerId: string, playerA: string, playerB: string) =>
    request<DirectMessage[]>(`/players/${playerA}/dm/${playerB}`, {
      method: 'GET',
      body: JSON.stringify({ player_id: callerId }),
    }),

  /**
   * Returns the number of unread DMs for playerId.
   * Caller must match playerId.
   * GET /players/{playerID}/dm/unread
   */
  unreadCount: (playerId: string) =>
    request<{ count: number }>(`/players/${playerId}/dm/unread`, {
      method: 'GET',
      body: JSON.stringify({ player_id: playerId }),
    }),

  /**
   * Marks a DM as read. No-op if already read.
   * POST /dm/{messageID}/read
   */
  markRead: (callerId: string, messageId: string) =>
    request<void>(`/dm/${messageId}/read`, {
      method: 'POST',
      body: JSON.stringify({ player_id: callerId }),
    }),

  /**
   * Reports a DM. Caller must be one of the two participants.
   * POST /players/{playerID}/dm/{otherPlayerID}/report
   */
  report: (callerId: string, playerA: string, playerB: string, messageId: string) =>
    request<void>(`/players/${playerA}/dm/${playerB}/report`, {
      method: 'POST',
      body: JSON.stringify({ player_id: callerId, message_id: messageId }),
    }),
}

// --- Notifications -----------------------------------------------------------

export const notifications = {
  /**
   * Lists notifications for playerId.
   * Pass include_read=true to include already-read notifications.
   * Read notifications older than 30 days are always excluded.
   * GET /players/{playerID}/notifications
   */
  list: (playerId: string, includeRead = false) =>
    request<Notification[]>(
      `/players/${playerId}/notifications?include_read=${includeRead}&player_id=${playerId}`,
    ),

  /**
   * Marks a notification as read. No-op if already read.
   * POST /notifications/{notificationID}/read
   */
  markRead: (playerId: string, notificationId: string) =>
    request<void>(`/notifications/${notificationId}/read`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),

  /**
   * Accepts the action associated with a notification (friend_request,
   * room_invitation). Returns 410 if the action has expired.
   * POST /notifications/{notificationID}/accept
   */
  accept: (playerId: string, notificationId: string) =>
    request<void>(`/notifications/${notificationId}/accept`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),

  /**
   * Declines the action associated with a notification.
   * POST /notifications/{notificationID}/decline
   */
  decline: (playerId: string, notificationId: string) =>
    request<void>(`/notifications/${notificationId}/decline`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
}

// --- Queue -------------------------------------------------------------------

export const queue = {
  /**
   * Adds the player to the ranked matchmaking queue.
   * Returns position and estimated wait time.
   * POST /queue
   */
  join: (playerId: string) =>
    request<QueuePosition>('/queue', {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),

  /**
   * Removes the player from the ranked matchmaking queue. Idempotent.
   * DELETE /queue
   */
  leave: (playerId: string) =>
    request<void>('/queue', {
      method: 'DELETE',
      body: JSON.stringify({ player_id: playerId }),
    }),

  /**
   * Records that the player accepts the proposed match.
   * If both players accept, match_ready is broadcast via the player channel.
   * POST /queue/accept
   */
  accept: (playerId: string, matchId: string) =>
    request<void>('/queue/accept', {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, match_id: matchId }),
    }),

  /**
   * Records that the player declines the proposed match.
   * The declining player is dropped from the queue and receives a penalty.
   * POST /queue/decline
   */
  decline: (playerId: string, matchId: string) =>
    request<void>('/queue/decline', {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, match_id: matchId }),
    }),
}

// --- Bot profiles ------------------------------------------------------------

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

// --- Utilities ---------------------------------------------------------------

/**
 * Returns the WebSocket URL for a room, including the player_id query param.
 * The server uses player_id to determine whether the connecting client is a
 * participant (in room_players) or a spectator.
 */
export function wsRoomUrl(roomId: string, playerId: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${window.location.host}/ws/rooms/${roomId}?player_id=${encodeURIComponent(playerId)}`
}

/**
 * Returns the WebSocket URL for the player's personal channel.
 * Connect once on login to receive queue, DM, and notification events.
 */
export function wsPlayerUrl(playerId: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${window.location.host}/ws/players/${encodeURIComponent(playerId)}`
}