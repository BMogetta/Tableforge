import { describe, it, expect, vi, beforeEach } from 'vitest'
import { renderHook, act, waitFor } from '@testing-library/react'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { createElement, type ReactNode } from 'react'
import { useBlockPlayer } from '../useBlockPlayer'

const mockBlock = vi.fn()
const mockUnblock = vi.fn()
const mockShowInfo = vi.fn()
const mockShowError = vi.fn()

vi.mock('@/features/friends/api', () => ({
  blocks: {
    block: (...args: unknown[]) => mockBlock(...args),
    unblock: (...args: unknown[]) => mockUnblock(...args),
  },
}))

vi.mock('@/stores/store', () => ({
  useAppStore: (selector: (s: unknown) => unknown) =>
    selector({ player: { id: 'player-1', username: 'alice' } }),
}))

vi.mock('@/ui/Toast', () => ({
  useToast: () => ({ showInfo: mockShowInfo, showError: mockShowError }),
}))

let qc: QueryClient

function wrapper({ children }: { children: ReactNode }) {
  return createElement(QueryClientProvider, { client: qc }, children)
}

beforeEach(() => {
  vi.clearAllMocks()
  qc = new QueryClient({
    defaultOptions: { queries: { retry: false }, mutations: { retry: false } },
  })
})

describe('useBlockPlayer', () => {
  it('starts with empty blocked list', () => {
    const { result } = renderHook(() => useBlockPlayer(), { wrapper })
    expect(result.current.blockedPlayers).toEqual([])
  })

  it('isBlocked returns false for unknown player', () => {
    const { result } = renderHook(() => useBlockPlayer(), { wrapper })
    expect(result.current.isBlocked('other-id')).toBe(false)
  })

  it('block adds player to local cache and shows toast', async () => {
    mockBlock.mockResolvedValue({})
    const { result } = renderHook(() => useBlockPlayer(), { wrapper })

    act(() => {
      result.current.block({ targetId: 'target-1', username: 'bob', avatarUrl: 'http://img' })
    })

    await waitFor(() => {
      expect(result.current.isBlocked('target-1')).toBe(true)
    })

    expect(mockBlock).toHaveBeenCalledWith('player-1', 'target-1')
    expect(mockShowInfo).toHaveBeenCalledWith('bob has been blocked.')
    expect(result.current.blockedPlayers).toHaveLength(1)
    expect(result.current.blockedPlayers[0].username).toBe('bob')
  })

  it('block invalidates friends queries', async () => {
    mockBlock.mockResolvedValue({})
    const invalidateSpy = vi.spyOn(qc, 'invalidateQueries')
    const { result } = renderHook(() => useBlockPlayer(), { wrapper })

    act(() => {
      result.current.block({ targetId: 'target-2', username: 'carol' })
    })

    await waitFor(() => {
      expect(invalidateSpy).toHaveBeenCalled()
    })

    const calls = invalidateSpy.mock.calls.map(c => c[0])
    expect(calls).toEqual(
      expect.arrayContaining([
        expect.objectContaining({ queryKey: ['friends', 'player-1'] }),
        expect.objectContaining({ queryKey: ['friends', 'player-1', 'pending'] }),
      ]),
    )
  })

  it('block does not duplicate on repeated block', async () => {
    mockBlock.mockResolvedValue({})
    const { result } = renderHook(() => useBlockPlayer(), { wrapper })

    act(() => {
      result.current.block({ targetId: 'target-1', username: 'bob' })
    })
    await waitFor(() => expect(result.current.isBlocked('target-1')).toBe(true))

    act(() => {
      result.current.block({ targetId: 'target-1', username: 'bob' })
    })
    await waitFor(() => expect(result.current.blockedPlayers).toHaveLength(1))
  })

  it('unblock removes player from local cache and shows toast', async () => {
    mockBlock.mockResolvedValue({})
    mockUnblock.mockResolvedValue(undefined)
    const { result } = renderHook(() => useBlockPlayer(), { wrapper })

    // Block first
    act(() => {
      result.current.block({ targetId: 'target-1', username: 'bob' })
    })
    await waitFor(() => expect(result.current.isBlocked('target-1')).toBe(true))

    // Unblock
    act(() => {
      result.current.unblock('target-1')
    })
    await waitFor(() => {
      expect(result.current.isBlocked('target-1')).toBe(false)
    })

    expect(mockUnblock).toHaveBeenCalledWith('player-1', 'target-1')
    expect(mockShowInfo).toHaveBeenCalledWith('bob has been unblocked.')
  })

  it('block error shows toast', async () => {
    mockBlock.mockRejectedValue(new Error('network fail'))
    const { result } = renderHook(() => useBlockPlayer(), { wrapper })

    act(() => {
      result.current.block({ targetId: 'target-1', username: 'bob' })
    })

    await waitFor(() => {
      expect(mockShowError).toHaveBeenCalled()
    })
  })

  it('unblock error shows toast', async () => {
    mockUnblock.mockRejectedValue(new Error('network fail'))
    const { result } = renderHook(() => useBlockPlayer(), { wrapper })

    act(() => {
      result.current.unblock('target-1')
    })

    await waitFor(() => {
      expect(mockShowError).toHaveBeenCalled()
    })
  })
})
