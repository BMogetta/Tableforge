export type WsEventType =
  | 'move_applied'
  | 'game_over'
  | 'player_joined'
  | 'player_left'
  | 'game_started'

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

  constructor(private roomId: string) {}

  connect() {
    if (this.ws) return
    const url = `${location.protocol === 'https:' ? 'wss' : 'ws'}://${location.host}/ws/rooms/${this.roomId}`
    this.ws = new WebSocket(url)

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
        this.reconnectTimer = setTimeout(() => this.connect(), 2000)
      }
    }
  }

  on(handler: Handler) {
    this.handlers.add(handler)
    return () => this.handlers.delete(handler)
  }

  close() {
    this.closed = true
    if (this.reconnectTimer) clearTimeout(this.reconnectTimer)
    this.ws?.close()
    this.ws = null
  }
}
