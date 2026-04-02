// All types mirror the Go store models.

import { GameSessionDTO } from './api-generated'
import { FontSize, SkinId } from './skins'
import { tracer } from './telemetry'
import { SpanKind, SpanStatusCode, context, propagation } from '@opentelemetry/api'

// =============================================================================
// CONSTANTS — avoids magic strings across the codebase.
// Pattern: const object + type alias from keyof typeof.
// Never use the string literals directly — always reference the const.
// =============================================================================

export const PlayerRole = {
  Player: 'player',
  Manager: 'manager',
  Owner: 'owner',
} as const
export type PlayerRole = (typeof PlayerRole)[keyof typeof PlayerRole]

export const RoomStatus = {
  Waiting: 'waiting',
  InProgress: 'in_progress',
  Finished: 'finished',
} as const
export type RoomStatus = (typeof RoomStatus)[keyof typeof RoomStatus]

export const SessionMode = {
  Casual: 'casual',
  Ranked: 'ranked',
} as const
export type SessionMode = (typeof SessionMode)[keyof typeof SessionMode]

export const ResultStatus = {
  Win: 'win',
  Draw: 'draw',
  Loss: 'loss',
} as const
export type ResultStatus = (typeof ResultStatus)[keyof typeof ResultStatus]

export const NotificationType = {
  FriendRequest: 'friend_request',
  RoomInvitation: 'room_invitation',
  BanIssued: 'ban_issued',
} as const
export type NotificationType = (typeof NotificationType)[keyof typeof NotificationType]

export const BanReason = {
  DeclineThreshold: 'decline_threshold',
  Moderator: 'moderator',
} as const
export type BanReason = (typeof BanReason)[keyof typeof BanReason]

export const AllowDMs = {
  Anyone: 'anyone',
  FriendsOnly: 'friends_only',
  Nobody: 'nobody',
} as const
export type AllowDMs = (typeof AllowDMs)[keyof typeof AllowDMs]

export const SettingType = {
  Select: 'select',
  Int: 'int',
} as const
export type SettingType = (typeof SettingType)[keyof typeof SettingType]

export const Language = {
  En: 'en',
  Es: 'es',
} as const
export type Language = (typeof Language)[keyof typeof Language]

// =============================================================================
// TYPES
// =============================================================================

export interface Player {
  id: string
  username: string
  role: PlayerRole
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
  role: PlayerRole
  note?: string
  invited_by?: string
  expires_at?: string
  created_at: string
}

export interface PlayerSettingMap {
  // Appearance
  theme?: SkinId
  language?: Language
  reduce_motion?: boolean
  font_size?: FontSize

  // Notifications
  notify_dm?: boolean
  notify_game_invite?: boolean
  notify_friend_request?: boolean
  notify_sound?: boolean

  // Audio (stubs — no audio system yet)
  mute_all?: boolean
  volume_master?: number // 0.0–1.0
  volume_sfx?: number
  volume_ui?: number
  volume_notifications?: number
  volume_music?: number

  // Gameplay
  show_move_hints?: boolean
  confirm_move?: boolean
  show_timer_warning?: boolean

  // Privacy
  show_online_status?: boolean
  allow_dms?: AllowDMs
}

export interface PlayerSettings {
  player_id: string
  settings: PlayerSettingMap
  updated_at: string
}

/** Canonical application defaults — mirrors store.DefaultPlayerSettings(). */
export const DEFAULT_SETTINGS: Required<PlayerSettingMap> = {
  theme: SkinId.Obsidian,
  language: Language.En,
  reduce_motion: false,
  font_size: FontSize.Medium,
  notify_dm: true,
  notify_game_invite: true,
  notify_friend_request: true,
  notify_sound: true,
  mute_all: false,
  volume_master: 1.0,
  volume_sfx: 1.0,
  volume_ui: 1.0,
  volume_notifications: 1.0,
  volume_music: 1.0,
  show_move_hints: true,
  confirm_move: false,
  show_timer_warning: true,
  show_online_status: true,
  allow_dms: AllowDMs.Anyone,
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
  status: RoomStatus
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

/** Inline result returned by move/surrender endpoints — based on engine.Result. */
export interface MoveResult {
  status: ResultStatus
  winner_id?: string
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
  timestamp: string
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
  reason: BanReason
  expires_at?: string
}

export interface Notification {
  id: string
  player_id: string
  type: NotificationType
  payload:
    | NotificationPayloadFriendRequest
    | NotificationPayloadRoomInvitation
    | NotificationPayloadBanIssued
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
  type: SettingType
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

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const method = (init?.method ?? 'GET').toUpperCase()
  const url = BASE + path

  return tracer.startActiveSpan(`${method} ${url}`, { kind: SpanKind.CLIENT }, async span => {
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
        // Redirect to dedicated page on rate limit so the user gets clear feedback.
        if (res.status === 429) {
          window.location.href = '/rate-limited'
        }

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
  constructor(
    public status: number,
    message: string,
  ) {
    super(message)
  }
}

// --- Auth --------------------------------------------------------------------
// These routes hit /auth/* outside the /api/v1 base path so they bypass
// the request() helper. Tracing is handled automatically by FetchInstrumentation.
// The GitHub login is a browser redirect and cannot be traced.

export const auth = {
  me: () =>
    fetch('/auth/me', { credentials: 'include' }).then(async res => {
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

export interface MatchHistoryEntry {
  id: string
  session_id: string
  game_id: string
  outcome: 'win' | 'loss' | 'draw' | 'forfeit'
  ended_by: string
  duration_secs?: number
  created_at: string
}

export interface MatchHistoryResponse {
  matches: MatchHistoryEntry[]
  total: number
}

export interface PlayerProfile {
  player_id: string
  bio?: string
  country?: string
  updated_at: string
}

export const players = {
  stats: (id: string) => request<PlayerStats>(`/players/${id}/stats`),
  sessions: (id: string) => request<GameSessionDTO[]>(`/players/${id}/sessions`),
  matches: (id: string, limit = 20, offset = 0) =>
    request<MatchHistoryResponse>(`/players/${id}/matches?limit=${limit}&offset=${offset}`),
  profile: (id: string) => request<PlayerProfile>(`/players/${id}/profile`),
}

// --- Player settings ---------------------------------------------------------

export const playerSettings = {
  get: (playerId: string) => request<PlayerSettings>(`/players/${playerId}/settings`),
  update: (playerId: string, settings: PlayerSettingMap) =>
    request<PlayerSettings>(`/players/${playerId}/settings`, {
      method: 'PUT',
      body: JSON.stringify(settings),
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
