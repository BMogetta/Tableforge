// All types mirror the Go store models.

import { tracer } from './telemetry'
import { SpanStatusCode, context, propagation } from '@opentelemetry/api'

export interface Player {
  id: string
  username: string
  role: 'player' | 'manager' | 'owner'
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

export interface LeaderboardEntry {
  player_id: string
  username: string
  avatar_url?: string
  wins: number
  losses: number
  draws: number
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

export interface SessionEvent {
  id: string
  session_id: string
  type: string
  player_id?: string
  payload?: Record<string, unknown>
  occurred_at: string
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
// We don't use the request() helper for auth routes since they have some special handling (e.g. login redirects to GitHub) and we don't want them to be traced like normal API calls.
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
}

// --- WebSocket URL -----------------------------------------------------------

/**
 * Returns the WebSocket URL for a room, including the player_id query param.
 * The server uses player_id to determine whether the connecting client is a
 * participant (in room_players) or a spectator.
 */
export function wsRoomUrl(roomId: string, playerId: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${window.location.host}/ws/rooms/${roomId}?player_id=${encodeURIComponent(playerId)}`
}

// --- Sessions ----------------------------------------------------------------

export const sessions = {
  get: (id: string) =>
    request<{
      session: GameSession
      state: { current_player_id: string; data: unknown }
      result?: GameResult
    }>(`/sessions/${id}`),
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
  history: (id: string) => request<Move[]>(`/sessions/${id}/history`),
  move: (sessionId: string, playerId: string, payload: unknown) =>
    request<{
      session: GameSession
      state: { current_player_id: string; data: unknown }
      is_over: boolean
    }>(`/sessions/${sessionId}/move`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, payload }),
    }),
  events: (id: string) => request<SessionEvent[]>(`/sessions/${id}/events`),
}

// --- Leaderboard -------------------------------------------------------------

export const leaderboard = {
  get: (gameId?: string, limit = 20) => {
    const params = new URLSearchParams({ limit: String(limit) })
    if (gameId) params.set('game_id', gameId)
    return request<LeaderboardEntry[]>(`/leaderboard?${params}`)
  },
}

// --- Game Registry -----------------------------------------------------------

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