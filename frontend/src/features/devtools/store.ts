import { create } from 'zustand'
import type { WsEvent, WsEventType } from '../../lib/ws'

export const MAX_EVENTS = 200

const EventSource = {
  room: 'room',
  player: 'player',
} as const
type EventSource = (typeof EventSource)[keyof typeof EventSource]

export interface CapturedEvent {
  id: number
  timestamp: number
  source: EventSource
  type: WsEventType
  payload: unknown
}

interface WsDevtoolsState {
  events: CapturedEvent[]
  /** Capture an event from either socket. */
  capture: (source: CapturedEvent['source'], event: WsEvent) => void
  /** Remove all captured events. */
  clear: () => void
}

let counter = 0

/**
 * Global store for WebSocket event capture.
 *
 * Events are captured by hooking into RoomSocket and PlayerSocket listeners
 * via useWsDevtools(). The store is capped at MAX_EVENTS to avoid unbounded
 * memory growth — oldest events are dropped when the cap is reached.
 *
 * Only mounted in development — the hook and panel are no-ops in production.
 */
export const useWsDevtoolsStore = create<WsDevtoolsState>(set => ({
  events: [],

  capture: (source, event) =>
    set(state => {
      const next: CapturedEvent = {
        id: ++counter,
        timestamp: Date.now(),
        source,
        type: event.type,
        payload: event.payload,
      }
      const events = [...state.events, next]
      // Drop oldest events when cap is reached.
      return { events: events.length > MAX_EVENTS ? events.slice(-MAX_EVENTS) : events }
    }),

  clear: () => set({ events: [] }),
}))
