import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { GameSession } from '@/lib/schema-generated.zod'
import { useGameSession } from '../useGameSession'

// --- Mocks -------------------------------------------------------------------

const mockGetSession = vi.fn()
const mockGetRoom = vi.fn()

vi.mock('@/lib/api/sessions', () => ({
  sessions: {
    get: (...args: unknown[]) => mockGetSession(...args),
  },
}))

vi.mock('@/features/room/api', () => ({
  rooms: {
    get: (...args: unknown[]) => mockGetRoom(...args),
  },
}))

// --- Helpers -----------------------------------------------------------------

function makeSession(overrides: Partial<GameSession> = {}): GameSession {
  const now = new Date().toISOString()
  return {
    id: 'session-1',
    room_id: 'room-1',
    game_id: 'tictactoe',
    mode: 'casual',
    move_count: 5,
    suspend_count: 0,
    ready_players: [],
    last_move_at: now,
    started_at: now,
    ...overrides,
  }
}

// --- Tests -------------------------------------------------------------------

describe('useGameSession', () => {
  let qc: QueryClient
  const onGameOver = vi.fn()

  function wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: qc }, children)
  }

  beforeEach(() => {
    qc = new QueryClient({
      defaultOptions: {
        queries: { retry: false, refetchOnMount: false },
      },
    })
    mockGetSession.mockReset()
    mockGetRoom.mockReset()
    onGameOver.mockReset()
  })

  function render() {
    return renderHook(() => useGameSession({ sessionId: 'session-1', onGameOver }), { wrapper })
  }

  it('fetches session and returns data', async () => {
    const session = makeSession()
    mockGetSession.mockResolvedValue({
      session,
      state: { current_player_id: 'p1', data: {} },
    })
    mockGetRoom.mockResolvedValue({
      room: { id: 'room-1' },
      players: [{ id: 'p1', username: 'alice' }],
      settings: {},
    })

    const { result } = render()

    await waitFor(() => expect(result.current.session).not.toBeNull())
    expect(result.current.session?.id).toBe('session-1')
    expect(result.current.loadError).toBeNull()
  })

  it('calls onGameOver when session is already finished on load', async () => {
    mockGetSession.mockResolvedValue({
      session: makeSession({ finished_at: new Date().toISOString() }),
      state: { current_player_id: 'p1', data: {} },
      result: { winner_id: 'p1', is_draw: false },
    })
    mockGetRoom.mockResolvedValue({
      room: { id: 'room-1' },
      players: [],
      settings: {},
    })

    render()

    await waitFor(() => expect(onGameOver).toHaveBeenCalledWith('p1', false))
  })

  it('calls onGameOver with draw', async () => {
    mockGetSession.mockResolvedValue({
      session: makeSession({ finished_at: new Date().toISOString() }),
      state: {},
      result: { winner_id: null, is_draw: true },
    })
    mockGetRoom.mockResolvedValue({
      room: { id: 'room-1' },
      players: [],
      settings: {},
    })

    render()

    await waitFor(() => expect(onGameOver).toHaveBeenCalledWith(null, true))
  })

  it('does not call onGameOver when session is active', async () => {
    mockGetSession.mockResolvedValue({
      session: makeSession(),
      state: {},
    })
    mockGetRoom.mockResolvedValue({
      room: { id: 'room-1' },
      players: [],
      settings: {},
    })

    render()

    await waitFor(() => expect(mockGetSession).toHaveBeenCalled())
    expect(onGameOver).not.toHaveBeenCalled()
  })

  it('does not call onGameOver when finished but no result (WS handled it)', async () => {
    mockGetSession.mockResolvedValue({
      session: makeSession({ finished_at: new Date().toISOString() }),
      state: {},
      // No result — WS game_over already called onGameOver.
    })
    mockGetRoom.mockResolvedValue({
      room: { id: 'room-1' },
      players: [],
      settings: {},
    })

    render()

    await waitFor(() => expect(mockGetSession).toHaveBeenCalled())
    expect(onGameOver).not.toHaveBeenCalled()
  })

  it('returns loadError on fetch failure', async () => {
    mockGetSession.mockRejectedValue(new Error('network failure'))

    const { result } = render()

    await waitFor(() => expect(result.current.loadError).not.toBeNull())
    expect(result.current.loadError?.message).toBe('network failure')
  })
})
