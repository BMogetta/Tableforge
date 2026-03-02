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
  code: string
  game_id: string
  owner_id: string
  status: 'waiting' | 'in_progress' | 'finished'
  max_players: number
  created_at: string
}

export interface RoomView {
  room: Room
  players: Player[]
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

export interface GameInfo {
  id: string
  name: string
  min_players: number
  max_players: number
}

export const gameRegistry = {
  list: () => request<GameInfo[]>('/games'),
}

export interface GameConfig {
  game_id: string
  default_timeout_secs: number
  min_timeout_secs: number
  max_timeout_secs: number
  timeout_penalty: 'lose_turn' | 'lose_game'
}

export const gameConfig = {
  get: (gameId: string) =>
    request<GameConfig>(`/games/${gameId}/config`),
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
