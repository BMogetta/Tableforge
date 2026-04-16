import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act, cleanup, fireEvent } from '@testing-library/react'
import { GameLoading } from '../GameLoading'

// ---------------------------------------------------------------------------
// Mocks
// ---------------------------------------------------------------------------

const mockGatewayOff = vi.fn()
const mockGatewayOn = vi.fn(() => mockGatewayOff)
const mockGateway = { on: mockGatewayOn }

// Mock stores to behave like selector-based hooks returning fake state.
vi.mock('../../../stores/socketStore', () => ({
  useSocketStore: (selector: (s: unknown) => unknown) =>
    selector({
      gateway: mockGateway,
    }),
}))

vi.mock('../../../stores/roomStore', () => ({
  useRoomStore: (selector: (s: unknown) => unknown) =>
    selector({
      isSpectator: false,
    }),
}))

vi.mock('../../../lib/assets', () => ({
  loadAssets: vi.fn().mockResolvedValue(undefined),
}))

vi.mock('../../../games', () => ({
  getGameAssets: vi.fn().mockReturnValue([]),
}))

vi.mock('../../../lib/api/sessions', () => ({
  sessions: {
    ready: vi.fn().mockResolvedValue({ all_ready: false, ready_players: [], required: 2 }),
  },
}))

// ---------------------------------------------------------------------------
// Imports after mocks (so mocked modules are used)
// ---------------------------------------------------------------------------

import { sessions } from '@/lib/api/sessions'

const mockReady = sessions.ready as ReturnType<typeof vi.fn>

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function advanceSeconds(n: number) {
  for (let i = 0; i < n; i++) {
    act(() => vi.advanceTimersByTime(1000))
  }
}

async function advancePastLoading() {
  await act(async () => {
    vi.advanceTimersByTime(3000)
    await Promise.resolve()
  })
}

function captureSocketListener(): (event: unknown) => void {
  const calls = mockGatewayOn.mock.calls as unknown as Array<[(event: unknown) => void]>
  const last = calls[calls.length - 1]
  if (!last) throw new Error('No socket listener registered')
  return last[0]
}
function renderComponent(
  props?: Partial<{
    sessionId: string
    gameId: string
    onReady: () => void
    onTimeout: () => void
  }>,
) {
  const onReady = vi.fn()
  const onTimeout = vi.fn()
  const result = render(
    <GameLoading
      sessionId={props?.sessionId ?? 'session-abc'}
      gameId={props?.gameId ?? 'tictactoe'}
      onReady={props?.onReady ?? onReady}
      onTimeout={props?.onTimeout ?? onTimeout}
    />,
  )
  return { onReady, onTimeout, unmount: result.unmount }
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('GameLoading', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockReady.mockReset()
    mockGatewayOn.mockReset()
    mockGatewayOn.mockReturnValue(mockGatewayOff)
    mockReady.mockResolvedValue({ all_ready: false, ready_players: [], required: 2 })
  })

  afterEach(() => {
    cleanup()
    vi.runOnlyPendingTimers()
    vi.useRealTimers()
  })

  // ── Loading phase ──────────────────────────────────────────────────────────

  it('shows the game id during loading phase', () => {
    renderComponent({ gameId: 'rootaccess' })
    expect(screen.getByText('rootaccess')).toBeInTheDocument()
  })

  it('shows "Loading..." while assets are loading', () => {
    renderComponent()
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('does not show waiting text during loading phase', () => {
    renderComponent()
    expect(screen.queryByText(/waiting for opponent/i)).not.toBeInTheDocument()
  })

  // ── Transition to waiting ──────────────────────────────────────────────────

  it('transitions to waiting phase after MIN_LOADING_MS', async () => {
    renderComponent()
    await advancePastLoading()
    expect(screen.getByText(/waiting for opponent/i)).toBeInTheDocument()
  })

  it('sends POST /ready after assets load', async () => {
    renderComponent({ sessionId: 'sess-1' })
    await advancePastLoading()
    expect(mockReady).toHaveBeenCalledWith('sess-1')
  })

  it('sends POST /ready only once even if re-rendered', async () => {
    renderComponent()
    await advancePastLoading()
    await advancePastLoading()
    expect(mockReady).toHaveBeenCalledTimes(1)
  })

  it('calls onReady immediately if server responds all_ready: true', async () => {
    mockReady.mockResolvedValue({ all_ready: true, ready_players: ['player-123'], required: 1 })
    const { onReady } = renderComponent()
    await advancePastLoading()
    await act(async () => {
      await Promise.resolve()
    })
    expect(onReady).toHaveBeenCalledTimes(1)
  })

  // ── Waiting phase — countdown ──────────────────────────────────────────────

  it('shows 60s countdown in waiting phase', async () => {
    renderComponent()
    await advancePastLoading()
    expect(screen.getByText('60s')).toBeInTheDocument()
  })

  it('decrements countdown every second', async () => {
    renderComponent()
    await advancePastLoading()
    advanceSeconds(3)
    expect(screen.getByText('57s')).toBeInTheDocument()
  })

  it('shows Back to Lobby button in waiting phase', async () => {
    renderComponent()
    await advancePastLoading()
    expect(screen.getByText(/back to lobby/i)).toBeInTheDocument()
  })

  it('calls onTimeout when Back to Lobby is clicked', async () => {
    const { onTimeout } = renderComponent()
    await advancePastLoading()
    fireEvent.click(screen.getByText(/back to lobby/i))
    expect(onTimeout).toHaveBeenCalledTimes(1)
  })

  // ── Waiting phase — WS events ─────────────────────────────────────────────

  it('shows ready count when player_ready event received', async () => {
    renderComponent()
    await advancePastLoading()
    const listener = captureSocketListener()
    act(() => {
      listener({ type: 'player_ready', payload: { ready_players: ['player-123'], required: 2 } })
    })
    expect(screen.getByText('1 / 2 ready')).toBeInTheDocument()
  })

  it('calls onReady when game_ready WS event received', async () => {
    const { onReady } = renderComponent()
    await advancePastLoading()
    const listener = captureSocketListener()
    act(() => {
      listener({ type: 'game_ready', payload: { session_id: 'session-abc' } })
    })
    expect(onReady).toHaveBeenCalledTimes(1)
  })

  it('calls onTimeout when game_over WS event received during waiting', async () => {
    const { onTimeout } = renderComponent()
    await advancePastLoading()
    const listener = captureSocketListener()
    act(() => {
      listener({ type: 'game_over', payload: {} })
    })
    expect(onTimeout).toHaveBeenCalledTimes(1)
  })

  it('does not call onReady or onTimeout for unrelated WS events', async () => {
    const { onReady, onTimeout } = renderComponent()
    await advancePastLoading()
    const listener = captureSocketListener()
    act(() => {
      listener({ type: 'move_applied', payload: {} })
    })
    expect(onReady).not.toHaveBeenCalled()
    expect(onTimeout).not.toHaveBeenCalled()
  })

  // ── Cleanup ────────────────────────────────────────────────────────────────

  it('unregisters WS listener on unmount', async () => {
    const { unmount } = renderComponent({ sessionId: 's' })
    await advancePastLoading()
    unmount()
    expect(mockGatewayOff).toHaveBeenCalled()
  })
})
