import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'

// Mock ws module before importing store
vi.mock('@/lib/ws', () => {
  class MockRoomSocket {
    url: string
    connected = false
    closed = false
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
  }

  class MockPlayerSocket {
    url: string
    connected = false
    closed = false
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
  }

  return { RoomSocket: MockRoomSocket, PlayerSocket: MockPlayerSocket }
})

// Mock skins to avoid DOM side effects
vi.mock('@/lib/skins', () => ({
  applySkin: vi.fn(),
  applyFontSize: vi.fn(),
  SkinId: { Obsidian: 'obsidian', Parchment: 'parchment' },
  FontSize: { Small: 'small', Medium: 'medium', Large: 'large' },
}))

import { useAppStore, QueueStatus } from '@/stores/store'
import { DEFAULT_SETTINGS } from '@/lib/api'
import type { Player } from '@/lib/schema-generated.zod'

const initialState = useAppStore.getState()

beforeEach(() => {
  // Reset store to initial state
  useAppStore.setState({
    player: null,
    socket: null,
    activeRoomId: null,
    playerSocket: null,
    isSpectator: false,
    spectatorCount: 0,
    presenceMap: {},
    queueStatus: QueueStatus.Idle,
    queueJoinedAt: null,
    matchId: null,
    dmTarget: null,
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
    role: 'user',
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
    useAppStore.getState().setIsSpectator(true)
    expect(useAppStore.getState().isSpectator).toBe(true)
  })

  it('sets spectator flag to false', () => {
    useAppStore.getState().setIsSpectator(true)
    useAppStore.getState().setIsSpectator(false)
    expect(useAppStore.getState().isSpectator).toBe(false)
  })
})

describe('setSpectatorCount', () => {
  it('sets spectator count', () => {
    useAppStore.getState().setSpectatorCount(5)
    expect(useAppStore.getState().spectatorCount).toBe(5)
  })

  it('resets to zero', () => {
    useAppStore.getState().setSpectatorCount(5)
    useAppStore.getState().setSpectatorCount(0)
    expect(useAppStore.getState().spectatorCount).toBe(0)
  })
})

// ---------------------------------------------------------------------------
// Presence
// ---------------------------------------------------------------------------

describe('setPlayerPresence', () => {
  it('sets a player as online', () => {
    useAppStore.getState().setPlayerPresence('p1', true)
    expect(useAppStore.getState().presenceMap).toEqual({ p1: true })
  })

  it('sets a player as offline', () => {
    useAppStore.getState().setPlayerPresence('p1', true)
    useAppStore.getState().setPlayerPresence('p1', false)
    expect(useAppStore.getState().presenceMap.p1).toBe(false)
  })

  it('tracks multiple players independently', () => {
    useAppStore.getState().setPlayerPresence('p1', true)
    useAppStore.getState().setPlayerPresence('p2', false)
    useAppStore.getState().setPlayerPresence('p3', true)

    const map = useAppStore.getState().presenceMap
    expect(map).toEqual({ p1: true, p2: false, p3: true })
  })
})

// ---------------------------------------------------------------------------
// Queue state machine
// ---------------------------------------------------------------------------

describe('queue state', () => {
  it('starts idle', () => {
    expect(useAppStore.getState().queueStatus).toBe(QueueStatus.Idle)
    expect(useAppStore.getState().queueJoinedAt).toBeNull()
    expect(useAppStore.getState().matchId).toBeNull()
  })

  it('setQueued transitions to Queued with timestamp', () => {
    const now = Date.now()
    useAppStore.getState().setQueued(now)

    expect(useAppStore.getState().queueStatus).toBe(QueueStatus.Queued)
    expect(useAppStore.getState().queueJoinedAt).toBe(now)
    expect(useAppStore.getState().matchId).toBeNull()
  })

  it('setMatchFound transitions to MatchFound with matchId', () => {
    useAppStore.getState().setQueued(Date.now())
    useAppStore.getState().setMatchFound('match-123')

    expect(useAppStore.getState().queueStatus).toBe(QueueStatus.MatchFound)
    expect(useAppStore.getState().matchId).toBe('match-123')
  })

  it('clearQueue resets to idle', () => {
    useAppStore.getState().setQueued(Date.now())
    useAppStore.getState().setMatchFound('match-123')
    useAppStore.getState().clearQueue()

    expect(useAppStore.getState().queueStatus).toBe(QueueStatus.Idle)
    expect(useAppStore.getState().queueJoinedAt).toBeNull()
    expect(useAppStore.getState().matchId).toBeNull()
  })

  it('setQueued clears any previous matchId', () => {
    useAppStore.getState().setMatchFound('old-match')
    useAppStore.getState().setQueued(Date.now())
    expect(useAppStore.getState().matchId).toBeNull()
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
  it('sets activeRoomId and creates socket', () => {
    useAppStore.getState().joinRoom('room-1', 'wss://example.com/ws/rooms/room-1')

    expect(useAppStore.getState().activeRoomId).toBe('room-1')
    expect(useAppStore.getState().socket).not.toBeNull()
  })

  it('resets spectator state when joining a room', () => {
    useAppStore.setState({ isSpectator: true, spectatorCount: 5 })
    useAppStore.getState().joinRoom('room-1', 'wss://example.com/ws/rooms/room-1')

    expect(useAppStore.getState().isSpectator).toBe(false)
    expect(useAppStore.getState().spectatorCount).toBe(0)
  })

  it('resets presence map when joining a room', () => {
    useAppStore.setState({ presenceMap: { p1: true } })
    useAppStore.getState().joinRoom('room-1', 'wss://example.com/ws/rooms/room-1')

    expect(useAppStore.getState().presenceMap).toEqual({})
  })

  it('closes previous socket when switching rooms', () => {
    useAppStore.getState().joinRoom('room-1', 'wss://example.com/ws/rooms/room-1')
    const firstSocket = useAppStore.getState().socket as unknown as { closed: boolean }

    useAppStore.getState().joinRoom('room-2', 'wss://example.com/ws/rooms/room-2')

    expect(firstSocket.closed).toBe(true)
    expect(useAppStore.getState().activeRoomId).toBe('room-2')
  })
})

describe('leaveRoom', () => {
  it('clears socket and activeRoomId', () => {
    useAppStore.getState().joinRoom('room-1', 'wss://example.com/ws/rooms/room-1')
    useAppStore.getState().leaveRoom()

    expect(useAppStore.getState().socket).toBeNull()
    expect(useAppStore.getState().activeRoomId).toBeNull()
  })

  it('resets spectator and presence state', () => {
    useAppStore.setState({ isSpectator: true, spectatorCount: 3, presenceMap: { p1: true } })
    useAppStore.getState().joinRoom('room-1', 'wss://example.com/ws/rooms/room-1')
    useAppStore.getState().leaveRoom()

    expect(useAppStore.getState().isSpectator).toBe(false)
    expect(useAppStore.getState().spectatorCount).toBe(0)
    expect(useAppStore.getState().presenceMap).toEqual({})
  })

  it('closes the socket', () => {
    useAppStore.getState().joinRoom('room-1', 'wss://example.com/ws/rooms/room-1')
    const socket = useAppStore.getState().socket as unknown as { closed: boolean }
    useAppStore.getState().leaveRoom()

    expect(socket.closed).toBe(true)
  })
})

// ---------------------------------------------------------------------------
// Player socket
// ---------------------------------------------------------------------------

describe('connectPlayerSocket', () => {
  it('creates and connects a player socket', () => {
    useAppStore.getState().connectPlayerSocket('wss://example.com/ws/players/p1')

    const ps = useAppStore.getState().playerSocket as unknown as {
      connected: boolean
      url: string
    }
    expect(ps).not.toBeNull()
    expect(ps.connected).toBe(true)
    expect(ps.url).toBe('wss://example.com/ws/players/p1')
  })

  it('closes previous player socket when reconnecting', () => {
    useAppStore.getState().connectPlayerSocket('wss://example.com/ws/players/p1')
    const first = useAppStore.getState().playerSocket as unknown as { closed: boolean }

    useAppStore.getState().connectPlayerSocket('wss://example.com/ws/players/p1')
    expect(first.closed).toBe(true)
  })
})

describe('disconnectPlayerSocket', () => {
  it('closes and nulls the player socket', () => {
    useAppStore.getState().connectPlayerSocket('wss://example.com/ws/players/p1')
    const ps = useAppStore.getState().playerSocket as unknown as { closed: boolean }

    useAppStore.getState().disconnectPlayerSocket()

    expect(ps.closed).toBe(true)
    expect(useAppStore.getState().playerSocket).toBeNull()
  })

  it('is safe to call when no socket exists', () => {
    expect(() => useAppStore.getState().disconnectPlayerSocket()).not.toThrow()
    expect(useAppStore.getState().playerSocket).toBeNull()
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
