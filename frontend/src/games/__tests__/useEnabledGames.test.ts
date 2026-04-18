import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useEnabledGames } from '../useEnabledGames'

// SDK state, mutated per-test.
const sdkState = {
  flagsReady: true,
  toggles: [] as Array<{ name: string; enabled: boolean }>,
}

vi.mock('@unleash/proxy-client-react', () => ({
  useFlagsStatus: () => ({ flagsReady: sdkState.flagsReady, flagsError: null }),
  useUnleashClient: () => ({
    getAllToggles: () => sdkState.toggles,
    isEnabled: (name: string) => {
      const t = sdkState.toggles.find(x => x.name === name)
      return t ? t.enabled : false
    },
  }),
}))

const mockGamesList = vi.fn()
vi.mock('@/features/lobby/api', () => ({
  gameRegistry: {
    list: () => mockGamesList(),
  },
}))

describe('useEnabledGames', () => {
  let qc: QueryClient

  function wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: qc }, children)
  }

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    sdkState.flagsReady = true
    sdkState.toggles = []
    mockGamesList.mockReset()
  })

  it('keeps every game when flags are not yet ready (cold start)', async () => {
    sdkState.flagsReady = false
    mockGamesList.mockResolvedValueOnce([
      { id: 'tictactoe', name: 'TicTacToe' },
      { id: 'rootaccess', name: 'Root Access' },
    ])

    const { result } = renderHook(() => useEnabledGames(), { wrapper })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.games.map(g => g.id)).toEqual(['tictactoe', 'rootaccess'])
  })

  it('hides games whose flag is OFF', async () => {
    sdkState.toggles = [
      { name: 'game-tictactoe-enabled', enabled: true },
      { name: 'game-rootaccess-enabled', enabled: false },
    ]
    mockGamesList.mockResolvedValueOnce([
      { id: 'tictactoe', name: 'TicTacToe' },
      { id: 'rootaccess', name: 'Root Access' },
    ])

    const { result } = renderHook(() => useEnabledGames(), { wrapper })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.games.map(g => g.id)).toEqual(['tictactoe'])
  })

  it('keeps games with no matching flag (default enabled)', async () => {
    // toggles is empty — neither game has a flag registered server-side yet.
    sdkState.toggles = []
    mockGamesList.mockResolvedValueOnce([{ id: 'new-game', name: 'New Game' }])

    const { result } = renderHook(() => useEnabledGames(), { wrapper })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.games.map(g => g.id)).toEqual(['new-game'])
  })

  it('hides every game when every flag is OFF', async () => {
    sdkState.toggles = [
      { name: 'game-tictactoe-enabled', enabled: false },
      { name: 'game-rootaccess-enabled', enabled: false },
    ]
    mockGamesList.mockResolvedValueOnce([
      { id: 'tictactoe', name: 'TicTacToe' },
      { id: 'rootaccess', name: 'Root Access' },
    ])

    const { result } = renderHook(() => useEnabledGames(), { wrapper })

    await waitFor(() => expect(result.current.isLoading).toBe(false))
    expect(result.current.games).toEqual([])
  })
})
