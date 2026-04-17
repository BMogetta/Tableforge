import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { act, renderHook, waitFor } from '@testing-library/react'
import { createElement, type ReactNode } from 'react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { usePauseResume } from '../usePauseResume'

// --- Mocks -------------------------------------------------------------------

const mockPause = vi.fn()
const mockResume = vi.fn()

vi.mock('@/lib/api/sessions', () => ({
  sessions: {
    pause: (...args: unknown[]) => mockPause(...args),
    resume: (...args: unknown[]) => mockResume(...args),
  },
}))

vi.mock('@/ui/Toast', () => ({
  useToast: () => ({ showError: vi.fn() }),
}))

vi.mock('@/utils/errors', () => ({
  catchToAppError: (e: unknown) => e,
}))

// --- Tests -------------------------------------------------------------------

describe('usePauseResume', () => {
  let qc: QueryClient

  function wrapper({ children }: { children: ReactNode }) {
    return createElement(QueryClientProvider, { client: qc }, children)
  }

  beforeEach(() => {
    qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
    mockPause.mockReset()
    mockResume.mockReset()
  })

  function render() {
    return renderHook(() => usePauseResume({ sessionId: 'session-1' }), { wrapper })
  }

  // --- Initial state ---------------------------------------------------------

  it('starts with all defaults', () => {
    const { result } = render()

    expect(result.current.isSuspended).toBe(false)
    expect(result.current.pauseVotes).toEqual([])
    expect(result.current.pauseRequired).toBe(0)
    expect(result.current.resumeVotes).toEqual([])
    expect(result.current.resumeRequired).toBe(0)
    expect(result.current.votedPause).toBe(false)
    expect(result.current.votedResume).toBe(false)
    expect(result.current.isPausePending).toBe(false)
    expect(result.current.isResumePending).toBe(false)
  })

  // --- WS event setters ------------------------------------------------------

  it('setPauseVoteUpdate updates votes and required', () => {
    const { result } = render()

    act(() => result.current.setPauseVoteUpdate(['p1', 'p2'], 3))

    expect(result.current.pauseVotes).toEqual(['p1', 'p2'])
    expect(result.current.pauseRequired).toBe(3)
  })

  it('setResumeVoteUpdate updates votes and required', () => {
    const { result } = render()

    act(() => result.current.setResumeVoteUpdate(['p1'], 2))

    expect(result.current.resumeVotes).toEqual(['p1'])
    expect(result.current.resumeRequired).toBe(2)
  })

  it('onSessionSuspended sets suspended and clears pause votes', () => {
    const { result } = render()

    act(() => result.current.setPauseVoteUpdate(['p1'], 2))
    act(() => result.current.onSessionSuspended())

    expect(result.current.isSuspended).toBe(true)
    expect(result.current.pauseVotes).toEqual([])
  })

  it('onSessionResumed clears all vote state', () => {
    const { result } = render()

    // Simulate a full pause → resume cycle via setters.
    act(() => {
      result.current.onSessionSuspended()
      result.current.setResumeVoteUpdate(['p1'], 2)
    })

    act(() => result.current.onSessionResumed())

    expect(result.current.isSuspended).toBe(false)
    expect(result.current.resumeVotes).toEqual([])
    expect(result.current.votedPause).toBe(false)
    expect(result.current.votedResume).toBe(false)
  })

  it('setSuspended can be set externally (initial fetch)', () => {
    const { result } = render()

    act(() => result.current.setSuspended(true))

    expect(result.current.isSuspended).toBe(true)
  })

  // --- votePause mutation ----------------------------------------------------

  it('votePause calls sessions.pause and sets votedPause', async () => {
    mockPause.mockResolvedValue({ all_voted: false, votes: 1, required: 2 })
    const { result } = render()

    act(() => result.current.votePause())

    await waitFor(() => expect(result.current.votedPause).toBe(true))
    expect(mockPause).toHaveBeenCalledWith('session-1')
    expect(result.current.isSuspended).toBe(false)
  })

  it('votePause with all_voted suspends session', async () => {
    mockPause.mockResolvedValue({ all_voted: true, votes: 2, required: 2 })
    const { result } = render()

    act(() => result.current.votePause())

    await waitFor(() => expect(result.current.isSuspended).toBe(true))
    expect(result.current.pauseVotes).toEqual([])
  })

  // --- voteResume mutation ---------------------------------------------------

  it('voteResume calls sessions.resume and sets votedResume', async () => {
    mockResume.mockResolvedValue({ all_voted: false, votes: 1, required: 2 })
    const { result } = render()

    act(() => result.current.setSuspended(true))
    act(() => result.current.voteResume())

    await waitFor(() => expect(result.current.votedResume).toBe(true))
    expect(mockResume).toHaveBeenCalledWith('session-1')
    expect(result.current.isSuspended).toBe(true)
  })

  it('voteResume with all_voted resumes and clears all state', async () => {
    mockResume.mockResolvedValue({ all_voted: true, votes: 2, required: 2 })
    const { result } = render()

    act(() => result.current.setSuspended(true))
    act(() => result.current.voteResume())

    await waitFor(() => expect(result.current.isSuspended).toBe(false))
    expect(result.current.resumeVotes).toEqual([])
    expect(result.current.votedPause).toBe(false)
    expect(result.current.votedResume).toBe(false)
  })
})
