import { describe, it, expect, vi, beforeEach } from 'vitest'

// Mock ws module before importing store
vi.mock('@/lib/ws', () => {
  class MockGatewaySocket {
    url: string
    connected = false
    closed = false
    subscribedRoom: string | null = null
    constructor(url: string) {
      this.url = url
    }
    connect() {
      this.connected = true
    }
    close() {
      this.closed = true
    }
    on() {
      return () => {}
    }
    subscribeRoom(roomId: string) {
      this.subscribedRoom = roomId
    }
    unsubscribeRoom() {
      this.subscribedRoom = null
    }
  }

  return { GatewaySocket: MockGatewaySocket }
})

// Mock skins to avoid DOM side effects
vi.mock('@/lib/skins', () => ({
  applySkin: vi.fn(),
  applyFontSize: vi.fn(),
  SkinId: { Obsidian: 'obsidian', Parchment: 'parchment' },
  FontSize: { Small: 'small', Medium: 'medium', Large: 'large' },
}))

import { useAppStore } from '@/stores/store'
import { useSocketStore } from '@/stores/socketStore'
import { useRoomStore } from '@/stores/roomStore'
import { useQueueStore, QueueStatus } from '@/stores/queueStore'
import { useSettingsStore } from '@/stores/settingsStore'
import { DEFAULT_SETTINGS } from '@/lib/api'
import type { Player } from '@/lib/schema-generated.zod'

beforeEach(() => {
  // Reset all stores to initial state
  useAppStore.setState({
    player: null,
    dmTarget: null,
  })
  useSocketStore.setState({
    gateway: null,
  })
  useRoomStore.setState({
    activeRoomId: null,
    isSpectator: false,
    spectatorCount: 0,
    presenceMap: {},
  })
  useQueueStore.setState({
    queueStatus: QueueStatus.Idle,
    queueJoinedAt: null,
    matchId: null,
  })
  useSettingsStore.setState({
    settings: { ...DEFAULT_SETTINGS },
  })
})

// ---------------------------------------------------------------------------
// Player state
// ---------------------------------------------------------------------------

describe('setPlayer', () => {
  const alice: Player = {
    id: 'p1',
    username: 'Alice',
    avatar_url: 'https://example.com/alice.png',
    role: 'player',
    is_bot: false,
    created_at: '2025-01-01T00:00:00Z',
  }

  it('sets player', () => {
    useAppStore.getState().setPlayer(alice)
    expect(useAppStore.getState().player).toEqual(alice)
  })

  it('clears player with null', () => {
    useAppStore.getState().setPlayer(alice)
    useAppStore.getState().setPlayer(null)
    expect(useAppStore.getState().player).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Spectator state
// ---------------------------------------------------------------------------

describe('setIsSpectator', () => {
  it('sets spectator flag to true', () => {
    useRoomStore.getState().setIsSpectator(true)
    expect(useRoomStore.getState().isSpectator).toBe(true)
  })

  it('sets spectator flag to false', () => {
    useRoomStore.getState().setIsSpectator(true)
    useRoomStore.getState().setIsSpectator(false)
    expect(useRoomStore.getState().isSpectator).toBe(false)
  })
})

describe('setSpectatorCount', () => {
  it('sets spectator count', () => {
    useRoomStore.getState().setSpectatorCount(5)
    expect(useRoomStore.getState().spectatorCount).toBe(5)
  })

  it('resets to zero', () => {
    useRoomStore.getState().setSpectatorCount(5)
    useRoomStore.getState().setSpectatorCount(0)
    expect(useRoomStore.getState().spectatorCount).toBe(0)
  })
})

// ---------------------------------------------------------------------------
// Presence
// ---------------------------------------------------------------------------

describe('setPlayerPresence', () => {
  it('sets a player as online', () => {
    useRoomStore.getState().setPlayerPresence('p1', true)
    expect(useRoomStore.getState().presenceMap).toEqual({ p1: true })
  })

  it('sets a player as offline', () => {
    useRoomStore.getState().setPlayerPresence('p1', true)
    useRoomStore.getState().setPlayerPresence('p1', false)
    expect(useRoomStore.getState().presenceMap.p1).toBe(false)
  })

  it('tracks multiple players independently', () => {
    useRoomStore.getState().setPlayerPresence('p1', true)
    useRoomStore.getState().setPlayerPresence('p2', false)
    useRoomStore.getState().setPlayerPresence('p3', true)

    const map = useRoomStore.getState().presenceMap
    expect(map).toEqual({ p1: true, p2: false, p3: true })
  })
})

// ---------------------------------------------------------------------------
// Queue state machine
// ---------------------------------------------------------------------------

describe('queue state', () => {
  it('starts idle', () => {
    expect(useQueueStore.getState().queueStatus).toBe(QueueStatus.Idle)
    expect(useQueueStore.getState().queueJoinedAt).toBeNull()
    expect(useQueueStore.getState().matchId).toBeNull()
  })

  it('setQueued transitions to Queued with timestamp', () => {
    const now = Date.now()
    useQueueStore.getState().setQueued(now)

    expect(useQueueStore.getState().queueStatus).toBe(QueueStatus.Queued)
    expect(useQueueStore.getState().queueJoinedAt).toBe(now)
    expect(useQueueStore.getState().matchId).toBeNull()
  })

  it('setMatchFound transitions to MatchFound with matchId', () => {
    useQueueStore.getState().setQueued(Date.now())
    useQueueStore.getState().setMatchFound('match-123')

    expect(useQueueStore.getState().queueStatus).toBe(QueueStatus.MatchFound)
    expect(useQueueStore.getState().matchId).toBe('match-123')
  })

  it('clearQueue resets to idle', () => {
    useQueueStore.getState().setQueued(Date.now())
    useQueueStore.getState().setMatchFound('match-123')
    useQueueStore.getState().clearQueue()

    expect(useQueueStore.getState().queueStatus).toBe(QueueStatus.Idle)
    expect(useQueueStore.getState().queueJoinedAt).toBeNull()
    expect(useQueueStore.getState().matchId).toBeNull()
  })

  it('setQueued clears any previous matchId', () => {
    useQueueStore.getState().setMatchFound('old-match')
    useQueueStore.getState().setQueued(Date.now())
    expect(useQueueStore.getState().matchId).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// DM target
// ---------------------------------------------------------------------------

describe('setDmTarget', () => {
  it('sets a DM target', () => {
    useAppStore.getState().setDmTarget('p2')
    expect(useAppStore.getState().dmTarget).toBe('p2')
  })

  it('clears DM target with null', () => {
    useAppStore.getState().setDmTarget('p2')
    useAppStore.getState().setDmTarget(null)
    expect(useAppStore.getState().dmTarget).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Room socket (joinRoom / leaveRoom)
// ---------------------------------------------------------------------------

describe('joinRoom', () => {
  it('sets activeRoomId and subscribes on gateway', () => {
    // Connect gateway first so joinRoom can subscribe
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    useRoomStore.getState().joinRoom('room-1')

    expect(useRoomStore.getState().activeRoomId).toBe('room-1')
    const gw = useSocketStore.getState().gateway as unknown as { subscribedRoom: string | null }
    expect(gw.subscribedRoom).toBe('room-1')
  })

  it('resets spectator state when joining a room', () => {
    useRoomStore.setState({ isSpectator: true, spectatorCount: 5 })
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    useRoomStore.getState().joinRoom('room-1')

    expect(useRoomStore.getState().isSpectator).toBe(false)
    expect(useRoomStore.getState().spectatorCount).toBe(0)
  })

  it('resets presence map when joining a room', () => {
    useRoomStore.setState({ presenceMap: { p1: true } })
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    useRoomStore.getState().joinRoom('room-1')

    expect(useRoomStore.getState().presenceMap).toEqual({})
  })

  it('unsubscribes previous room when switching rooms', () => {
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    useRoomStore.getState().joinRoom('room-1')
    useRoomStore.getState().joinRoom('room-2')

    expect(useRoomStore.getState().activeRoomId).toBe('room-2')
    const gw = useSocketStore.getState().gateway as unknown as { subscribedRoom: string | null }
    expect(gw.subscribedRoom).toBe('room-2')
  })
})

describe('leaveRoom', () => {
  it('clears activeRoomId and unsubscribes from gateway', () => {
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    useRoomStore.getState().joinRoom('room-1')
    useRoomStore.getState().leaveRoom()

    expect(useRoomStore.getState().activeRoomId).toBeNull()
    const gw = useSocketStore.getState().gateway as unknown as { subscribedRoom: string | null }
    expect(gw.subscribedRoom).toBeNull()
  })

  it('resets spectator and presence state', () => {
    useRoomStore.setState({ isSpectator: true, spectatorCount: 3, presenceMap: { p1: true } })
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    useRoomStore.getState().joinRoom('room-1')
    useRoomStore.getState().leaveRoom()

    expect(useRoomStore.getState().isSpectator).toBe(false)
    expect(useRoomStore.getState().spectatorCount).toBe(0)
    expect(useRoomStore.getState().presenceMap).toEqual({})
  })

  it('does not close the gateway socket', () => {
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    useRoomStore.getState().joinRoom('room-1')
    useRoomStore.getState().leaveRoom()

    const gw = useSocketStore.getState().gateway as unknown as { closed: boolean }
    expect(gw.closed).toBe(false)
    expect(useSocketStore.getState().gateway).not.toBeNull()
  })
})

// ---------------------------------------------------------------------------
// Gateway socket
// ---------------------------------------------------------------------------

describe('connectGateway', () => {
  it('creates and connects a gateway socket', () => {
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')

    const gw = useSocketStore.getState().gateway as unknown as {
      connected: boolean
      url: string
    }
    expect(gw).not.toBeNull()
    expect(gw.connected).toBe(true)
    expect(gw.url).toBe('wss://example.com/ws/players/p1')
  })

  it('skips reconnect when URL is the same', () => {
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    const first = useSocketStore.getState().gateway

    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    expect(useSocketStore.getState().gateway).toBe(first)
  })

  it('closes previous gateway socket when URL changes', () => {
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    const first = useSocketStore.getState().gateway as unknown as { closed: boolean }

    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p2')
    expect(first.closed).toBe(true)
  })
})

describe('disconnectGateway', () => {
  it('closes and nulls the gateway socket', () => {
    useSocketStore.getState().connectGateway('wss://example.com/ws/players/p1')
    const gw = useSocketStore.getState().gateway as unknown as { closed: boolean }

    useSocketStore.getState().disconnectGateway()

    expect(gw.closed).toBe(true)
    expect(useSocketStore.getState().gateway).toBeNull()
  })

  it('is safe to call when no socket exists', () => {
    expect(() => useSocketStore.getState().disconnectGateway()).not.toThrow()
    expect(useSocketStore.getState().gateway).toBeNull()
  })
})

// ---------------------------------------------------------------------------
// QueueStatus constants
// ---------------------------------------------------------------------------

describe('QueueStatus', () => {
  it('has expected values', () => {
    expect(QueueStatus.Idle).toBe('idle')
    expect(QueueStatus.Queued).toBe('queued')
    expect(QueueStatus.MatchFound).toBe('match_found')
  })
})
