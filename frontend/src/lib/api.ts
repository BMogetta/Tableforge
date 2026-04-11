// All types mirror the Go store models.

import { isDev } from '@/lib/env'
import { z } from 'zod'
import {
  gameSessionSchema,
  getPlayerStatsResponseSchema,
  listAchievementsResponseSchema,
  listPlayerMatchesResponseSchema,
  playerProfileSchema,
  playerSettingsSchema,
} from './schema-generated.zod'
import type { Player, RoomPlayer, PlayerSettingMap, Room } from './schema-generated.zod'
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
  FriendRequestAccepted: 'friend_request_accepted',
  RoomInvitation: 'room_invitation',
  BanIssued: 'ban_issued',
  AchievementUnlocked: 'achievement_unlocked',
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
// TYPES — re-exports from generated schemas
// =============================================================================

export type {
  Player,
  PlayerAchievement,
  RoomPlayer,
  PlayerSettingMap,
  PlayerSettings,
  PlayerProfile,
  AllowedEmail,
  Notification,
  Room,
} from './schema-generated.zod'

export interface RoomView {
  room: Room
  players: RoomPlayer[]
  // Key/value map of all settings for this room.
  // Always present — defaults are inserted when the room is created.
  settings: Record<string, string>
  // Live spectator count. Kept in sync by spectator_joined / spectator_left
  // WS events in Room.tsx; may be absent on initial HTTP fetch.
  spectator_count?: number
  // Present when room status is "in_progress" — the active game session ID.
  active_session_id?: string
}

// --- Notification payload subtypes (frontend-only cast targets) --------------

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

// --- Lobby settings types ----------------------------------------------------

export type { LobbySetting, GameInfo } from './schema-generated.zod'

// --- HTTP client -------------------------------------------------------------

const BASE = '/api/v1'

/**
 * Attempts to refresh the access token using the refresh cookie.
 * Returns true if successful, false otherwise.
 */
let refreshPromise: Promise<boolean> | null = null

async function tryRefresh(): Promise<boolean> {
  // Deduplicate concurrent refresh attempts (multiple 401s at once).
  if (refreshPromise) return refreshPromise

  refreshPromise = fetch('/auth/refresh', {
    method: 'POST',
    credentials: 'include',
  })
    .then(res => res.ok)
    .catch(() => false)
    .finally(() => {
      refreshPromise = null
    })

  return refreshPromise
}

export async function request<T>(path: string, init?: RequestInit): Promise<T> {
  const method = (init?.method ?? 'GET').toUpperCase()
  const url = BASE + path

  return tracer.startActiveSpan(`${method} ${url}`, { kind: SpanKind.CLIENT }, async span => {
    const headers: Record<string, string> = {
      'Content-Type': 'application/json',
      ...(init?.headers as Record<string, string>),
    }
    propagation.inject(context.active(), headers)

    const doFetch = () =>
      fetch(url, {
        ...init,
        headers,
        credentials: 'include',
      })

    try {
      let res = await doFetch()

      span.setAttributes({
        'http.method': method,
        'http.url': url,
        'http.status_code': res.status,
      })

      // On 401 with token_expired, attempt a silent refresh then retry.
      if (res.status === 401) {
        const body = await res
          .clone()
          .json()
          .catch(() => null)
        if (body?.error === 'token_expired') {
          const refreshed = await tryRefresh()
          if (refreshed) {
            res = await doFetch()
            span.setAttribute('http.status_code', res.status)
          } else {
            window.location.href = '/login'
            span.setStatus({ code: SpanStatusCode.ERROR, message: 'session expired' })
            span.end()
            throw new ApiError(401, 'Session expired')
          }
        }
      }

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

/**
 * Like request(), but validates the response body against a Zod schema.
 * The schema drives the return type — no separate generic needed.
 *
 * On validation failure:
 * - Dev/test: throws SchemaError with full Zod details (caught by e2e tests)
 * - Production: throws SchemaError with generic message (no internals leaked)
 */
export async function validatedRequest<T extends z.ZodTypeAny>(
  schema: T,
  path: string,
  init?: RequestInit,
): Promise<z.infer<T>> {
  const data = await request<unknown>(path, init)
  const result = schema.safeParse(data)
  if (!result.success) {
    const details = isDev
      ? result.error.issues.map(i => `${i.path.join('.')}: ${i.message}`)
      : undefined
    const message = isDev
      ? `Response validation failed: ${path}`
      : 'Unexpected response from server'
    if (isDev) {
      console.error(`[SchemaError] ${path}`, result.error.issues)
    }
    throw new SchemaError(message, details)
  }
  return result.data
}

export class SchemaError extends Error {
  constructor(
    message: string,
    public details?: string[],
  ) {
    super(message)
    this.name = 'SchemaError'
  }
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
  me: async (): Promise<Player> => {
    let res = await fetch('/auth/me', { credentials: 'include' })

    // If access token expired, try refresh then retry.
    if (res.status === 401) {
      const body = await res
        .clone()
        .json()
        .catch(() => null)
      if (body?.error === 'token_expired') {
        const refreshed = await tryRefresh()
        if (refreshed) {
          res = await fetch('/auth/me', { credentials: 'include' })
        }
      }
    }

    if (!res.ok) {
      const body = await res.json().catch(() => ({}))
      throw new ApiError(res.status, body.error ?? res.statusText)
    }
    return res.json() as Promise<Player>
  },
  logout: () => fetch('/auth/logout', { method: 'POST', credentials: 'include' }),
  loginUrl: '/auth/github',
}

// --- Players -----------------------------------------------------------------

export type { MatchHistoryEntry } from './schema-generated.zod'

export const players = {
  stats: (id: string) => validatedRequest(getPlayerStatsResponseSchema, `/players/${id}/stats`),
  sessions: (id: string) => validatedRequest(z.array(gameSessionSchema), `/players/${id}/sessions`),
  matches: (id: string, limit = 20, offset = 0) =>
    validatedRequest(
      listPlayerMatchesResponseSchema,
      `/players/${id}/matches?limit=${limit}&offset=${offset}`,
    ),
  profile: (id: string) => validatedRequest(playerProfileSchema, `/players/${id}/profile`),
  achievements: (id: string) =>
    validatedRequest(listAchievementsResponseSchema, `/players/${id}/achievements`),
}

// --- Player settings ---------------------------------------------------------

export const playerSettings = {
  get: (playerId: string) =>
    validatedRequest(playerSettingsSchema, `/players/${playerId}/settings`),
  update: (playerId: string, settings: PlayerSettingMap) =>
    validatedRequest(playerSettingsSchema, `/players/${playerId}/settings`, {
      method: 'PUT',
      body: JSON.stringify(settings),
    }),
}

// --- Utilities ---------------------------------------------------------------

/**
 * Returns the WebSocket URL for a room.
 * Identity comes from the JWT cookie — no query params needed.
 * @param _playerId kept for caller compatibility; identity comes from JWT cookie.
 */
export function wsRoomUrl(roomId: string, _playerId?: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${window.location.host}/ws/rooms/${roomId}`
}

/**
 * Returns the WebSocket URL for the player's personal channel.
 * Connect once on login to receive queue, DM, and notification events.
 */
export function wsPlayerUrl(playerId: string): string {
  const proto = window.location.protocol === 'https:' ? 'wss' : 'ws'
  return `${proto}://${window.location.host}/ws/players/${encodeURIComponent(playerId)}`
}
