import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { GatewaySocket, type WsEvent } from '../ws'

// Stub dependencies
vi.mock('../telemetry', () => ({
  emitErrorLog: vi.fn(),
}))
vi.mock('../device', () => ({
  getDeviceContextAttrs: () => ({}),
}))

// --- Mock WebSocket ----------------------------------------------------------

class MockWebSocket {
  static instances: MockWebSocket[] = []
  static readonly OPEN = 1
  onopen: (() => void) | null = null
  onmessage: ((e: { data: string }) => void) | null = null
  onclose: ((e: { code: number }) => void) | null = null
  readyState = 0
  closed = false
  sent: string[] = []

  constructor(public url: string) {
    MockWebSocket.instances.push(this)
  }

  close() {
    this.closed = true
  }

  send(data: string) {
    this.sent.push(data)
  }

  // Test helpers
  simulateOpen() {
    this.readyState = 1
    this.onopen?.()
  }
  simulateMessage(data: unknown) {
    this.onmessage?.({ data: JSON.stringify(data) })
  }
  simulateClose(code = 1006) {
    this.readyState = 3
    this.onclose?.({ code })
  }
}

// --- Setup -------------------------------------------------------------------

let mockFetch: ReturnType<typeof vi.fn>

beforeEach(() => {
  vi.useFakeTimers()
  MockWebSocket.instances = []
  vi.stubGlobal('WebSocket', MockWebSocket)
  mockFetch = vi.fn()
  vi.stubGlobal('fetch', mockFetch)
  Object.defineProperty(window, 'location', {
    value: { href: '' },
    writable: true,
    configurable: true,
  })
})

afterEach(() => {
  vi.useRealTimers()
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

// --- GatewaySocket tests -----------------------------------------------------

describe('GatewaySocket', () => {
  it('emits ws_connected on open', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    const handler = vi.fn()
    socket.on(handler)
    socket.connect()

    MockWebSocket.instances[0].simulateOpen()

    expect(handler).toHaveBeenCalledWith({ type: 'ws_connected', payload: null })
  })

  it('forwards parsed messages to handlers', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    const handler = vi.fn()
    socket.on(handler)
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    const event: WsEvent = { type: 'move_applied', payload: {} as never }
    MockWebSocket.instances[0].simulateMessage(event)

    expect(handler).toHaveBeenCalledWith(event)
  })

  it('ignores malformed messages', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    const handler = vi.fn()
    socket.on(handler)
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    // Send raw non-JSON
    MockWebSocket.instances[0].onmessage?.({ data: 'not json{' })

    // Only ws_connected should have been called, no crash
    expect(handler).toHaveBeenCalledTimes(1)
  })

  it('reconnects with exponential backoff', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    const handler = vi.fn()
    socket.on(handler)
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    // First disconnect
    MockWebSocket.instances[0].simulateClose()
    expect(handler).toHaveBeenCalledWith({ type: 'ws_reconnecting', payload: null })

    // Should create new WS after 500ms (500 * 2^0)
    vi.advanceTimersByTime(500)
    expect(MockWebSocket.instances).toHaveLength(2)

    // Second disconnect
    MockWebSocket.instances[1].simulateClose()
    // Should wait 1000ms (500 * 2^1)
    vi.advanceTimersByTime(999)
    expect(MockWebSocket.instances).toHaveLength(2)
    vi.advanceTimersByTime(1)
    expect(MockWebSocket.instances).toHaveLength(3)
  })

  it('stops reconnecting after MAX_ATTEMPTS and emits ws_disconnected', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    const handler = vi.fn()
    socket.on(handler)
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    // Simulate 10 consecutive disconnects
    for (let i = 0; i < 10; i++) {
      const ws = MockWebSocket.instances[MockWebSocket.instances.length - 1]
      ws.simulateClose()
      vi.advanceTimersByTime(30_000) // enough for any backoff
    }

    const events = handler.mock.calls.map((c: unknown[]) => (c[0] as WsEvent).type)
    expect(events.filter(e => e === 'ws_disconnected')).toHaveLength(1)

    // No more WS instances created after the 10th failure
    const countBefore = MockWebSocket.instances.length
    vi.advanceTimersByTime(60_000)
    expect(MockWebSocket.instances).toHaveLength(countBefore)
  })

  it('resets attempt count on successful connection', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    // Disconnect + reconnect
    MockWebSocket.instances[0].simulateClose()
    vi.advanceTimersByTime(500)
    MockWebSocket.instances[1].simulateOpen() // success resets counter

    // Should be able to do 10 more attempts from scratch
    for (let i = 0; i < 9; i++) {
      MockWebSocket.instances[MockWebSocket.instances.length - 1].simulateClose()
      vi.advanceTimersByTime(30_000)
    }
    // After 9 failures we should still get a reconnect (not disconnected yet)
    expect(MockWebSocket.instances.length).toBeGreaterThan(10)
  })

  it('handles auth close (4001) with successful refresh', async () => {
    mockFetch.mockResolvedValue({ ok: true })
    const socket = new GatewaySocket('ws://test/gateway')
    const handler = vi.fn()
    socket.on(handler)
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    MockWebSocket.instances[0].simulateClose(4001)

    // Let the async refresh resolve
    await vi.advanceTimersByTimeAsync(0)

    expect(mockFetch).toHaveBeenCalledWith(
      '/auth/refresh',
      expect.objectContaining({ method: 'POST' }),
    )
    expect(handler).toHaveBeenCalledWith({ type: 'ws_reconnecting', payload: null })
    expect(MockWebSocket.instances).toHaveLength(2) // reconnected
  })

  it('handles auth close (4001) with failed refresh → redirects to login', async () => {
    mockFetch.mockResolvedValue({ ok: false })
    const socket = new GatewaySocket('ws://test/gateway')
    const handler = vi.fn()
    socket.on(handler)
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    MockWebSocket.instances[0].simulateClose(4001)

    await vi.advanceTimersByTimeAsync(0)

    expect(handler).toHaveBeenCalledWith({ type: 'ws_disconnected', payload: null })
    expect(window.location.href).toBe('/login')
  })

  it('handles auth close (1008) with redirect', async () => {
    mockFetch.mockResolvedValue({ ok: false })
    const socket = new GatewaySocket('ws://test/gateway')
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    MockWebSocket.instances[0].simulateClose(1008)
    await vi.advanceTimersByTimeAsync(0)

    expect(window.location.href).toBe('/login')
  })

  it('unsubscribe removes handler', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    const handler = vi.fn()
    const off = socket.on(handler)
    socket.connect()

    off()
    MockWebSocket.instances[0].simulateOpen()

    expect(handler).not.toHaveBeenCalled()
  })

  it('close prevents reconnection', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    socket.close()
    expect(MockWebSocket.instances[0].closed).toBe(true)

    // No reconnection timer fires
    vi.advanceTimersByTime(60_000)
    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('connect is idempotent when already connected', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    socket.connect()
    socket.connect() // should not create a second WS

    expect(MockWebSocket.instances).toHaveLength(1)
  })

  it('backoff is capped at MAX_BACKOFF_MS (30s)', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    // Simulate 8 disconnects to get high backoff
    for (let i = 0; i < 8; i++) {
      MockWebSocket.instances[MockWebSocket.instances.length - 1].simulateClose()
      vi.advanceTimersByTime(30_000)
    }

    const countBefore = MockWebSocket.instances.length
    MockWebSocket.instances[MockWebSocket.instances.length - 1].simulateClose()

    // Even at attempt 9, should reconnect within 30s
    vi.advanceTimersByTime(30_000)
    expect(MockWebSocket.instances.length).toBe(countBefore + 1)
  })

  // --- Room subscription -----------------------------------------------------

  it('subscribeRoom sends subscribe_room control message', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    socket.subscribeRoom('room-42')

    expect(MockWebSocket.instances[0].sent).toContainEqual(
      JSON.stringify({ action: 'subscribe_room', room_id: 'room-42' }),
    )
  })

  it('unsubscribeRoom sends unsubscribe_room control message', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    socket.subscribeRoom('room-42')
    socket.unsubscribeRoom()

    expect(MockWebSocket.instances[0].sent).toContainEqual(
      JSON.stringify({ action: 'unsubscribe_room' }),
    )
  })

  it('re-subscribes to active room on reconnect', () => {
    const socket = new GatewaySocket('ws://test/gateway')
    socket.connect()
    MockWebSocket.instances[0].simulateOpen()

    socket.subscribeRoom('room-7')

    // Disconnect + reconnect
    MockWebSocket.instances[0].simulateClose()
    vi.advanceTimersByTime(500)
    MockWebSocket.instances[1].simulateOpen()

    // The second WS should have sent a subscribe_room message on open
    expect(MockWebSocket.instances[1].sent).toContainEqual(
      JSON.stringify({ action: 'subscribe_room', room_id: 'room-7' }),
    )
  })
})
