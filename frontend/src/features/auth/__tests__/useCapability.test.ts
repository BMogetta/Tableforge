import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { renderHook, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useCapability } from '../useCapability'

const mockValidatedRequest = vi.fn()

vi.mock('@/lib/api', () => ({
  validatedRequest: (...args: unknown[]) => mockValidatedRequest(...args),
}))

describe('useCapability', () => {
  let qc: QueryClient

  function wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: qc }, children)
  }

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mockValidatedRequest.mockReset()
  })

  it('returns server capabilities on success', async () => {
    mockValidatedRequest.mockResolvedValueOnce({ canSeeDevtools: true })

    const { result } = renderHook(() => useCapability(), { wrapper })

    await waitFor(() => {
      expect(result.current.isLoading).toBe(false)
    })
    expect(result.current.capabilities.canSeeDevtools).toBe(true)
    expect(result.current.isError).toBe(false)
  })

  it('falls back to a zero-capability object on 401/error', async () => {
    mockValidatedRequest.mockRejectedValueOnce(new Error('unauthorized'))

    const { result } = renderHook(() => useCapability(), { wrapper })

    await waitFor(() => {
      expect(result.current.isError).toBe(true)
    })
    // Even with an error, we return EMPTY_CAPABILITIES so gated UI renders safely.
    expect(result.current.capabilities.canSeeDevtools).toBe(false)
  })

  it('returns isLoading=true before the request settles', () => {
    mockValidatedRequest.mockImplementation(() => new Promise(() => {}))

    const { result } = renderHook(() => useCapability(), { wrapper })

    expect(result.current.isLoading).toBe(true)
    expect(result.current.capabilities.canSeeDevtools).toBe(false)
  })
})
