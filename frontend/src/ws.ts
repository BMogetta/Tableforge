export type WsEventType =
  // Game flow
  | 'move_applied'
  | 'game_over'
  | 'game_started'
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

export interface WsEvent {
  type: WsEventType
  payload: unknown
}

type Handler = (event: WsEvent) => void

export class RoomSocket {
  private ws: WebSocket | null = null
  private handlers = new Set<Handler>()
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private closed = false
  private attemptCount = 0
  private static readonly MAX_ATTEMPTS = 10
  private static readonly MAX_BACKOFF_MS = 30_000

  /**
   * @param url Full WebSocket URL including any query params (e.g. player_id).
   *            Use wsRoomUrl() from api.ts to build this.
   */
  constructor(private url: string) {}

  connect() {
    if (this.ws) return
    this.ws = new WebSocket(this.url)

    this.ws.onopen = () => {
      this.attemptCount = 0
      this.emit('ws_connected')
    }

    this.ws.onmessage = (e) => {
      try {
        const event: WsEvent = JSON.parse(e.data)
        this.handlers.forEach((h) => h(event))
      } catch {
        // ignore malformed messages
      }
    }

    this.ws.onclose = () => {
      this.ws = null
      if (!this.closed) {
        this.attemptCount++
        if (this.attemptCount >= RoomSocket.MAX_ATTEMPTS) {
          this.emit('ws_disconnected')
        }
        this.emit('ws_reconnecting')
        const delay = Math.min(2 ** this.attemptCount * 1000, RoomSocket.MAX_BACKOFF_MS)
        this.reconnectTimer = setTimeout(() => this.connect(), delay)
      }
    }
  }

  on(handler: Handler): () => void {
    this.handlers.add(handler)
    return () => { this.handlers.delete(handler) }
  }

  close() {
    this.closed = true
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
    this.ws = null
  }

  private emit(type: WsEventType) {
    this.handlers.forEach((h) => h({ type, payload: null }))
  }
}

export class PlayerSocket {
  private ws: WebSocket | null = null
  private handlers = new Set<Handler>()
  private reconnectTimer: ReturnType<typeof setTimeout> | null = null
  private closed = false
  private attemptCount = 0
  private static readonly MAX_ATTEMPTS = 10
  private static readonly MAX_BACKOFF_MS = 30_000

  constructor(private url: string) {}

  connect() {
    if (this.ws) return
    this.ws = new WebSocket(this.url)

    this.ws.onopen = () => {
      this.attemptCount = 0
      this.emit('ws_connected')
    }

    this.ws.onmessage = (e) => {
      try {
        const event: WsEvent = JSON.parse(e.data)
        this.handlers.forEach((h) => h(event))
      } catch {
        // ignore malformed messages
      }
    }

    this.ws.onclose = () => {
      this.ws = null
      if (!this.closed) {
        this.attemptCount++
        if (this.attemptCount >= PlayerSocket.MAX_ATTEMPTS) {
          this.emit('ws_disconnected')
          return
        }
        this.emit('ws_reconnecting')
        const delay = Math.min(2 ** this.attemptCount * 1000, PlayerSocket.MAX_BACKOFF_MS)
        this.reconnectTimer = setTimeout(() => this.connect(), delay)
      }
    }
  }

  on(handler: Handler): () => void {
    this.handlers.add(handler)
    return () => { this.handlers.delete(handler) }
  }

  close() {
    this.closed = true
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
    this.ws = null
  }

  private emit(type: WsEventType) {
    this.handlers.forEach((h) => h({ type, payload: null }))
  }
}