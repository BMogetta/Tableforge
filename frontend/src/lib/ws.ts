import type { Notification, RoomView } from './api'
import { getDeviceContextAttrs } from './device'
import type { GameSession } from './schema-generated.zod'
import { emitErrorLog } from './telemetry'

/**
 * Attempts to refresh the access token. Used by GatewaySocket
 * when the server closes the connection due to auth failure.
 */
async function tryWsRefresh(): Promise<boolean> {
  try {
    const res = await fetch('/auth/refresh', { method: 'POST', credentials: 'include' })
    return res.ok
  } catch {
    return false
  }
}

export type WsEventType =
  // Control acks (server → client)
  | 'room_subscribed'
  // Game flow
  | 'game_started'
  | 'game_over'
  | 'move_applied'
  | 'player_ready'
  | 'game_ready'
  | 'rematch_vote'
  | 'rematch_ready'
  // Room
  | 'player_joined'
  | 'player_left'
  | 'owner_changed'
  | 'room_closed'
  | 'setting_updated'
  | 'presence_update'
  | 'spectator_joined'
  | 'spectator_left'
  // Chat
  | 'chat_message'
  | 'chat_message_hidden'
  // Direct messages
  | 'dm_received'
  | 'dm_read'
  // Pause / resume
  | 'pause_vote_update'
  | 'session_suspended'
  | 'resume_vote_update'
  | 'session_resumed'
  // Matchmaking queue
  | 'queue_joined'
  | 'queue_left'
  | 'match_found'
  | 'match_cancelled'
  | 'match_ready'
  // Notifications
  | 'notification_received'
  // Synthetic client-side connection events
  | 'ws_connected'
  | 'ws_reconnecting'
  | 'ws_disconnected'

// --- Payload types -----------------------------------------------------------

/** Mirrors the MoveResult shape returned by the runtime on move/surrender/game_over. */
export interface WsPayloadMoveResult {
  session: GameSession
  state: { current_player_id: string; data: unknown }
  is_over: boolean
  result?: {
    status?: 'win' | 'draw'
    winner_id?: string
    ended_by?: string
  }
}

export interface WsPayloadPlayerReady {
  ready_players: string[]
  required: number
  all_ready: boolean
}

export interface WsPayloadGameReady {
  session_id: string
}

export interface WsPayloadGameStarted {
  session: { id: string }
}

export interface WsPayloadRematchVote {
  votes: number
  total_players: number
}

export interface WsPayloadRematchReady {
  room_id: string
}

export interface WsPayloadOwnerChanged {
  owner_id: string
}

export interface WsPayloadSettingUpdated {
  key: string
  value: string
}

export interface WsPayloadPresenceUpdate {
  player_id: string
  online: boolean
}

export interface WsPayloadSpectatorCount {
  spectator_count: number
}

/** Chat message payload — note: uses message_id/timestamp, not id/created_at. */
export interface WsPayloadChatMessage {
  message_id: string
  room_id: string
  player_id: string
  content: string
  reported: boolean
  hidden: boolean
  timestamp: string
}

export interface WsPayloadChatMessageHidden {
  message_id: string
}

export interface WsPayloadQueueJoined {
  position: number
  estimated_wait_secs: number
  /** Present when the server re-queued the player after an opponent decline. */
  reason?: string
}

export interface WsPayloadQueueLeft {
  reason: string
}

export interface WsPayloadMatchFound {
  match_id: string
  quality: number
  timeout: number
}

export interface WsPayloadMatchCancelled {
  match_id: string
  reason: string
}

export interface WsPayloadMatchReady {
  room_id: string
  session_id: string
}

export interface WsPayloadPlayerJoined extends RoomView {}

export interface WsPayloadPlayerLeft {
  player_id: string
}

export interface WsPayloadDMReceived {
  message_id: string
  from: string
  content: string
  timestamp: string
}

export interface WsPayloadDMRead {
  message_id: string
}

export interface WsPayloadPauseVoteUpdate {
  votes: string[]
  required: number
}

export interface WsPayloadSessionSuspended {
  suspended_at: string
}

export interface WsPayloadResumeVoteUpdate {
  votes: string[]
  required: number
}

export interface WsPayloadSessionResumed {
  resumed_at: string
}

export interface WsPayloadNotificationReceived extends Notification {}

export interface WsPayloadRoomSubscribed {
  room_id: string
  spectator: boolean
}

// --- Discriminated union -----------------------------------------------------

export type WsEvent =
  | { type: 'room_subscribed'; payload: WsPayloadRoomSubscribed }
  | { type: 'game_started'; payload: WsPayloadGameStarted }
  | { type: 'game_over'; payload: WsPayloadMoveResult }
  | { type: 'player_ready'; payload: WsPayloadPlayerReady }
  | { type: 'game_ready'; payload: WsPayloadGameReady }
  | { type: 'move_applied'; payload: WsPayloadMoveResult }
  | { type: 'rematch_vote'; payload: WsPayloadRematchVote }
  | { type: 'rematch_ready'; payload: WsPayloadRematchReady }
  | { type: 'owner_changed'; payload: WsPayloadOwnerChanged }
  | { type: 'room_closed'; payload: null }
  | { type: 'player_joined'; payload: WsPayloadPlayerJoined }
  | { type: 'player_left'; payload: WsPayloadPlayerLeft }
  | { type: 'setting_updated'; payload: WsPayloadSettingUpdated }
  | { type: 'presence_update'; payload: WsPayloadPresenceUpdate }
  | { type: 'spectator_joined'; payload: WsPayloadSpectatorCount }
  | { type: 'spectator_left'; payload: WsPayloadSpectatorCount }
  | { type: 'chat_message'; payload: WsPayloadChatMessage }
  | { type: 'chat_message_hidden'; payload: WsPayloadChatMessageHidden }
  | { type: 'dm_received'; payload: WsPayloadDMReceived }
  | { type: 'dm_read'; payload: WsPayloadDMRead }
  | { type: 'pause_vote_update'; payload: WsPayloadPauseVoteUpdate }
  | { type: 'session_suspended'; payload: WsPayloadSessionSuspended }
  | { type: 'resume_vote_update'; payload: WsPayloadResumeVoteUpdate }
  | { type: 'session_resumed'; payload: WsPayloadSessionResumed }
  | { type: 'queue_joined'; payload: WsPayloadQueueJoined }
  | { type: 'queue_left'; payload: WsPayloadQueueLeft }
  | { type: 'match_found'; payload: WsPayloadMatchFound }
  | { type: 'match_cancelled'; payload: WsPayloadMatchCancelled }
  | { type: 'match_ready'; payload: WsPayloadMatchReady }
  | { type: 'notification_received'; payload: WsPayloadNotificationReceived }
  | { type: 'ws_connected'; payload: null }
  | { type: 'ws_reconnecting'; payload: null }
  | { type: 'ws_disconnected'; payload: null }

type Handler = (event: WsEvent) => void

// --- GatewaySocket -----------------------------------------------------------

/**
 * GatewaySocket is a unified WebSocket connection that replaces both the old
 * PlayerSocket and RoomSocket. It connects once at login to
 * /ws/players/{playerID} and dynamically subscribes/unsubscribes from room
 * channels via control messages sent to the ws-gateway.
 *
 * All events (player-scoped and room-scoped) arrive on this single connection.
 */
export class GatewaySocket {
  private ws: WebSocket | null = null
  private handlers = new Set<Handler>()
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private closed = false
  private attemptCount = 0
  private static readonly MAX_ATTEMPTS = 10
  private static readonly MAX_BACKOFF_MS = 30_000

  /** The room ID the gateway is currently subscribed to, if any. */
  private activeRoomId: string | null = null

  readonly url: string

  constructor(url: string) {
    this.url = url
  }

  connect() {
    if (this.ws) return
    this.ws = new WebSocket(this.url)

    this.ws.onopen = () => {
      this.attemptCount = 0
      this.emitSynthetic('ws_connected')
      // Re-subscribe to the room that was active before reconnect.
      if (this.activeRoomId) {
        this.send({ action: 'subscribe_room', room_id: this.activeRoomId })
      }
    }

    this.ws.onmessage = e => {
      try {
        const event: WsEvent = JSON.parse(e.data)
        this.handlers.forEach(h => h(event))
      } catch {
        // ignore malformed messages
      }
    }

    this.ws.onclose = e => {
      this.ws = null
      if (this.closed) return
      if (e.code === 4001 || e.code === 1008) {
        this.handleAuthClose()
        return
      }
      this.attemptCount++
      if (this.attemptCount >= GatewaySocket.MAX_ATTEMPTS) {
        emitErrorLog('WebSocket connection lost after max retries', {
          'error.type': 'ws.disconnect',
          'ws.url': this.url,
          'ws.attempts': String(this.attemptCount),
          'ws.socket_type': 'gateway',
          ...getDeviceContextAttrs(),
        })
        this.emitSynthetic('ws_disconnected')
        return
      }
      this.emitSynthetic('ws_reconnecting')
      const delay = Math.min(500 * 2 ** (this.attemptCount - 1), GatewaySocket.MAX_BACKOFF_MS)
      this.reconnectTimer = setTimeout(() => this.connect(), delay)
    }
  }

  private async handleAuthClose() {
    const refreshed = await tryWsRefresh()
    if (refreshed) {
      this.emitSynthetic('ws_reconnecting')
      this.connect()
    } else {
      this.emitSynthetic('ws_disconnected')
      window.location.href = '/login'
    }
  }

  /** Subscribe to a room channel. Sends a control message to the server. */
  subscribeRoom(roomId: string) {
    this.activeRoomId = roomId
    this.send({ action: 'subscribe_room', room_id: roomId })
  }

  /** Unsubscribe from the current room channel. */
  unsubscribeRoom() {
    if (!this.activeRoomId) return
    this.send({ action: 'unsubscribe_room' })
    this.activeRoomId = null
  }

  on(handler: Handler): () => void {
    this.handlers.add(handler)
    return () => {
      this.handlers.delete(handler)
    }
  }

  close() {
    this.closed = true
    this.activeRoomId = null
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
    this.ws = null
  }

  private send(msg: Record<string, unknown>) {
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify(msg))
    }
  }

  private emitSynthetic(type: 'ws_connected' | 'ws_reconnecting' | 'ws_disconnected') {
    this.handlers.forEach(h => h({ type, payload: null }))
  }
}
