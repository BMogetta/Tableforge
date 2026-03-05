export type WsEventType =
  | 'move_applied'
  | 'game_over'
  | 'player_joined'
  | 'player_left'
  | 'game_started'
  | 'rematch_vote'
  | 'rematch_ready'
  | 'rematch_started'
  | 'owner_changed'
  | 'setting_updated'
  | 'room_closed'
  | 'spectator_joined'
  | 'spectator_left'
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
        // After 3 failed attempts with no success, emit disconnected.
        if (this.attemptCount >= 3) {
          this.emit('ws_disconnected')
        } else {
          this.emit('ws_reconnecting')
        }
        this.reconnectTimer = setTimeout(() => this.connect(), 2000)
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