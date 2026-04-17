import { act, renderHook } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { useClipboard } from '../useClipboard'

// ---------------------------------------------------------------------------
// Setup
// ---------------------------------------------------------------------------

const mockWriteText = vi.fn()

beforeEach(() => {
  vi.useFakeTimers()
  Object.defineProperty(navigator, 'clipboard', {
    value: { writeText: mockWriteText },
    writable: true,
    configurable: true,
  })
})

afterEach(() => {
  vi.runAllTimers()
  vi.useRealTimers()
  vi.clearAllMocks()
})

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('useClipboard', () => {
  // ── Success ───────────────────────────────────────────────────────────────

  it('returns [null, true] on success', async () => {
    mockWriteText.mockResolvedValue(undefined)
    const { result } = renderHook(() => useClipboard())

    let copyResult: Awaited<ReturnType<typeof result.current.copy>>
    await act(async () => {
      copyResult = await result.current.copy('hello')
    })

    expect(copyResult![0]).toBeNull()
    expect(copyResult![1]).toBe(true)
  })

  it('calls navigator.clipboard.writeText with the provided text', async () => {
    mockWriteText.mockResolvedValue(undefined)
    const { result } = renderHook(() => useClipboard())

    await act(async () => {
      await result.current.copy('room-code-ABC')
    })

    expect(mockWriteText).toHaveBeenCalledWith('room-code-ABC')
  })

  it('sets copied to true after successful copy', async () => {
    mockWriteText.mockResolvedValue(undefined)
    const { result } = renderHook(() => useClipboard())

    await act(async () => {
      await result.current.copy('text')
    })

    expect(result.current.copied).toBe(true)
  })

  it('resets copied to false after resetMs', async () => {
    mockWriteText.mockResolvedValue(undefined)
    const { result } = renderHook(() => useClipboard(1500))

    await act(async () => {
      await result.current.copy('text')
    })
    expect(result.current.copied).toBe(true)

    act(() => vi.advanceTimersByTime(1500))
    expect(result.current.copied).toBe(false)
  })

  it('does not reset copied before resetMs elapses', async () => {
    mockWriteText.mockResolvedValue(undefined)
    const { result } = renderHook(() => useClipboard(2000))

    await act(async () => {
      await result.current.copy('text')
    })

    act(() => vi.advanceTimersByTime(1000))
    expect(result.current.copied).toBe(true)
  })

  it('uses 2000ms as default resetMs', async () => {
    mockWriteText.mockResolvedValue(undefined)
    const { result } = renderHook(() => useClipboard())

    await act(async () => {
      await result.current.copy('text')
    })

    act(() => vi.advanceTimersByTime(1999))
    expect(result.current.copied).toBe(true)

    act(() => vi.advanceTimersByTime(1))
    expect(result.current.copied).toBe(false)
  })

  // ── Failure ───────────────────────────────────────────────────────────────

  it('returns [ClipboardError, null] when clipboard throws', async () => {
    mockWriteText.mockRejectedValue(new Error('Permission denied'))
    const { result } = renderHook(() => useClipboard())

    let copyResult: Awaited<ReturnType<typeof result.current.copy>>
    await act(async () => {
      copyResult = await result.current.copy('text')
    })

    expect(copyResult![0]).toMatchObject({
      reason: 'CLIPBOARD_UNAVAILABLE',
      message: 'Failed to copy to clipboard',
    })
    expect(copyResult![1]).toBeNull()
  })

  it('does not set copied to true on failure', async () => {
    mockWriteText.mockRejectedValue(new Error('Not allowed'))
    const { result } = renderHook(() => useClipboard())

    await act(async () => {
      await result.current.copy('text')
    })

    expect(result.current.copied).toBe(false)
  })

  it('copied stays false after failure even after resetMs', async () => {
    mockWriteText.mockRejectedValue(new Error('Not allowed'))
    const { result } = renderHook(() => useClipboard(500))

    await act(async () => {
      await result.current.copy('text')
    })

    act(() => vi.advanceTimersByTime(500))
    expect(result.current.copied).toBe(false)
  })

  // ── Idempotency ───────────────────────────────────────────────────────────

  it('can copy multiple times sequentially', async () => {
    mockWriteText.mockResolvedValue(undefined)
    const { result } = renderHook(() => useClipboard(500))

    await act(async () => {
      await result.current.copy('first')
    })
    act(() => vi.advanceTimersByTime(500))
    expect(result.current.copied).toBe(false)

    await act(async () => {
      await result.current.copy('second')
    })
    expect(result.current.copied).toBe(true)
    expect(mockWriteText).toHaveBeenCalledTimes(2)
    expect(mockWriteText).toHaveBeenLastCalledWith('second')
  })
})
