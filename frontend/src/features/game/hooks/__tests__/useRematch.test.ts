import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useRematch } from '../useRematch'

// --- Mocks -------------------------------------------------------------------

const mockRematch = vi.fn()
const mockNavigate = vi.fn()

vi.mock('@/lib/api/sessions', () => ({
  sessions: {
    rematch: (...args: unknown[]) => mockRematch(...args),
  },
}))

vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mockNavigate,
}))

vi.mock('@/ui/Toast', () => ({
  useToast: () => ({ showError: vi.fn() }),
}))

vi.mock('@/utils/errors', () => ({
  catchToAppError: (e: unknown) => e,
}))

// --- Tests -------------------------------------------------------------------

describe('useRematch', () => {
  let qc: QueryClient

  function wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: qc }, children)
  }

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mockRematch.mockReset()
    mockNavigate.mockReset()
  })

  function render() {
    return renderHook(() => useRematch({ sessionId: 'session-1' }), { wrapper })
  }

  // --- Initial state ---------------------------------------------------------

  it('starts with defaults', () => {
    const { result } = render()

    expect(result.current.rematchVotes).toBe(0)
    expect(result.current.totalPlayers).toBe(0)
    expect(result.current.votedRematch).toBe(false)
    expect(result.current.isPending).toBe(false)
  })

  // --- WS event setters ------------------------------------------------------

  it('onRematchVote updates votes and total', () => {
    const { result } = render()

    act(() => result.current.onRematchVote(1, 2))

    expect(result.current.rematchVotes).toBe(1)
    expect(result.current.totalPlayers).toBe(2)
  })

  it('onRematchReady navigates to room', () => {
    const { result } = render()

    act(() => result.current.onRematchReady('room-42'))

    expect(mockNavigate).toHaveBeenCalledWith({
      to: '/rooms/$roomId',
      params: { roomId: 'room-42' },
    })
  })

  it('onRematchGameStarted navigates to game', () => {
    const { result } = render()

    act(() => result.current.onRematchGameStarted('session-99'))

    expect(mockNavigate).toHaveBeenCalledWith({
      to: '/game/$sessionId',
      params: { sessionId: 'session-99' },
    })
  })

  // --- voteRematch mutation --------------------------------------------------

  it('voteRematch calls API and updates state', async () => {
    mockRematch.mockResolvedValue({ votes: 1, total_players: 2 })
    const { result } = render()

    act(() => result.current.voteRematch())

    await waitFor(() => expect(result.current.votedRematch).toBe(true))
    expect(mockRematch).toHaveBeenCalledWith('session-1')
    expect(result.current.rematchVotes).toBe(1)
    expect(result.current.totalPlayers).toBe(2)
  })
})
