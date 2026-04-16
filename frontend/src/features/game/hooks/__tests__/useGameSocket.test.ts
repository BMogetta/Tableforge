import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createElement, type ReactNode } from 'react'
import { useGameSocket } from '../useGameSocket'
import type { GatewaySocket, WsEvent } from '@/lib/ws'
import { keys } from '@/lib/queryClient'
import type { GameSession } from '@/lib/schema-generated.zod'

// --- Mocks -------------------------------------------------------------------

vi.mock('@/lib/turn-notifier', () => ({ notifyTurn: vi.fn() }))
vi.mock('@/lib/sfx', () => ({ sfx: { play: vi.fn() } }))
vi.mock('@/stores/store', () => ({
  useAppStore: { getState: () => ({ player: { id: 'local-player' } }) },
}))

// --- Helpers -----------------------------------------------------------------

type OnHandler = (event: WsEvent) => void

function createMockSocket() {
  let handler: OnHandler | null = null
  return {
    on: vi.fn((h: OnHandler) => {
      handler = h
      return () => { handler = null }
    }),
    emit(event: WsEvent) {
      handler?.(event)
    },
  }
}

function createCallbacks() {
  return {
    pauseResume: {
      setPauseVoteUpdate: vi.fn(),
      setResumeVoteUpdate: vi.fn(),
      onSessionSuspended: vi.fn(),
      onSessionResumed: vi.fn(),
    },
    rematch: {
      onRematchVote: vi.fn(),
      onRematchReady: vi.fn(),
      onRematchGameStarted: vi.fn(),
    },
    onGameOver: vi.fn(),
  }
}

function makeSession(moveCount: number): GameSession {
  const now = new Date().toISOString()
  return {
    id: 'session-1',
    room_id: 'room-1',
    game_id: 'tictactoe',
    mode: 'casual',
    move_count: moveCount,
    suspend_count: 0,
    ready_players: [],
    last_move_at: now,
    started_at: now,
  }
}

function makeMovePayload(moveCount: number, currentPlayerId = 'other-player') {
  return {
    session: makeSession(moveCount),
    state: { current_player_id: currentPlayerId, data: {} },
    is_over: false,
  }
}

function makeGameOverPayload(winnerId: string | undefined, isDraw: boolean) {
  return {
    session: makeSession(5),
    state: { current_player_id: '', data: {} },
    is_over: true,
    result: {
      winner_id: winnerId,
      status: isDraw ? ('draw' as const) : ('win' as const),
    },
  }
}

// Cache helper type — the hook writes is_over to the cache even though
// SessionCache doesn't declare it.
interface CacheWithIsOver {
  session: GameSession
  state: unknown
  is_over?: boolean
  result?: { winner_id?: string; is_draw?: boolean } | null
}

// --- Tests -------------------------------------------------------------------

describe('useGameSocket', () => {
  let qc: QueryClient
  let gateway: ReturnType<typeof createMockSocket>
  let cbs: ReturnType<typeof createCallbacks>

  function wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: qc }, children)
  }

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    gateway = createMockSocket()
    cbs = createCallbacks()
  })

  function seedCache(moveCount: number) {
    qc.setQueryData(keys.session('session-1'), {
      session: makeSession(moveCount),
      state: { current_player_id: 'p1', data: {} },
    })
  }

  function getCache() {
    return qc.getQueryData<CacheWithIsOver>(keys.session('session-1'))
  }

  function render() {
    return renderHook(
      () =>
        useGameSocket({
          sessionId: 'session-1',
          gateway: gateway as unknown as GatewaySocket,
          ...cbs,
        }),
      { wrapper },
    )
  }

  // --- Deduplication ---------------------------------------------------------

  it('applies move_applied when move_count is newer', () => {
    seedCache(2)

    render()
    gateway.emit({ type: 'move_applied', payload: makeMovePayload(3) })

    expect(getCache()?.session.move_count).toBe(3)
  })

  it('ignores move_applied when move_count is stale', () => {
    seedCache(5)

    render()
    gateway.emit({ type: 'move_applied', payload: makeMovePayload(3) })

    expect(getCache()?.session.move_count).toBe(5)
  })

  it('ignores move_applied when move_count is equal', () => {
    seedCache(4)

    render()
    gateway.emit({ type: 'move_applied', payload: makeMovePayload(4) })

    expect(getCache()?.session.move_count).toBe(4)
  })

  it('applies move_applied when cache is empty (first event)', () => {
    render()
    gateway.emit({ type: 'move_applied', payload: makeMovePayload(1) })

    expect(getCache()?.session.move_count).toBe(1)
  })

  // --- game_over never deduplicated ------------------------------------------

  it('always applies game_over even when move_count is stale', () => {
    seedCache(10)

    render()
    gateway.emit({ type: 'game_over', payload: makeGameOverPayload('winner-1', false) })

    expect(getCache()?.is_over).toBe(true)
    expect(cbs.onGameOver).toHaveBeenCalledWith('winner-1', false)
  })

  it('handles game_over draw', () => {
    render()
    gateway.emit({ type: 'game_over', payload: makeGameOverPayload(undefined, true) })

    expect(cbs.onGameOver).toHaveBeenCalledWith(null, true)
  })

  // --- Event delegation ------------------------------------------------------

  it('delegates pause_vote_update', () => {
    render()
    gateway.emit({
      type: 'pause_vote_update',
      payload: { votes: ['p1'], required: 2 },
    })

    expect(cbs.pauseResume.setPauseVoteUpdate).toHaveBeenCalledWith(['p1'], 2)
  })

  it('delegates session_suspended', () => {
    render()
    gateway.emit({ type: 'session_suspended', payload: { suspended_at: new Date().toISOString() } })

    expect(cbs.pauseResume.onSessionSuspended).toHaveBeenCalled()
  })

  it('delegates resume_vote_update', () => {
    render()
    gateway.emit({
      type: 'resume_vote_update',
      payload: { votes: ['p1'], required: 2 },
    })

    expect(cbs.pauseResume.setResumeVoteUpdate).toHaveBeenCalledWith(['p1'], 2)
  })

  it('delegates session_resumed', () => {
    render()
    gateway.emit({ type: 'session_resumed', payload: { resumed_at: new Date().toISOString() } })

    expect(cbs.pauseResume.onSessionResumed).toHaveBeenCalled()
  })

  it('delegates rematch_vote', () => {
    render()
    gateway.emit({
      type: 'rematch_vote',
      payload: { votes: 1, total_players: 2 },
    })

    expect(cbs.rematch.onRematchVote).toHaveBeenCalledWith(1, 2)
  })

  it('delegates rematch_ready', () => {
    render()
    gateway.emit({
      type: 'rematch_ready',
      payload: { room_id: 'room-2' },
    })

    expect(cbs.rematch.onRematchReady).toHaveBeenCalledWith('room-2')
  })

  it('delegates game_started (rematch)', () => {
    render()
    gateway.emit({
      type: 'game_started',
      payload: { session: { id: 'session-2' } },
    })

    expect(cbs.rematch.onRematchGameStarted).toHaveBeenCalledWith('session-2')
  })

  // --- Cleanup ---------------------------------------------------------------

  it('unsubscribes on unmount', () => {
    const { unmount } = render()
    unmount()

    // After unmount, emitting should not update cache.
    gateway.emit({ type: 'move_applied', payload: makeMovePayload(99) })
    expect(getCache()).toBeUndefined()
  })

  // --- Null sockets ----------------------------------------------------------

  it('handles null socket gracefully', () => {
    renderHook(
      () =>
        useGameSocket({
          sessionId: 'session-1',
          gateway: null,
          ...cbs,
        }),
      { wrapper },
    )
    // No error thrown.
  })
})
