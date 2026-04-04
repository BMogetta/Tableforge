import { useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { sessions } from '@/lib/api/sessions'
import { keys } from '@/lib/queryClient'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'

interface UsePauseResumeOptions {
  sessionId: string
}

export interface PauseResumeState {
  isSuspended: boolean
  pauseVotes: string[]
  pauseRequired: number
  resumeVotes: string[]
  resumeRequired: number
  votedPause: boolean
  votedResume: boolean
  isPausePending: boolean
  isResumePending: boolean
  setSuspended: (value: boolean) => void
  setPauseVoteUpdate: (votes: string[], required: number) => void
  setResumeVoteUpdate: (votes: string[], required: number) => void
  onSessionSuspended: () => void
  onSessionResumed: () => void
  votePause: () => void
  voteResume: () => void
}

/**
 * Manages pause/resume voting state and mutations for a game session.
 *
 * Responsibilities:
 * - Tracks local voted state (votedPause, votedResume) to prevent double-voting.
 * - Exposes setters for WS event handlers in useGameSocket to update vote counts.
 * - Handles the pause/resume API mutations with toast error reporting.
 *
 * The isSuspended flag is the source of truth for whether the board is shown
 * or the suspended screen. It's set by:
 *   - Initial session fetch (synced externally via setSuspended)
 *   - WS session_suspended / session_resumed events (via onSessionSuspended/onSessionResumed)
 *
 * @testability
 * Mock `sessions.pause` and `sessions.resume` from `../../lib/api`.
 * Use renderHook from @testing-library/react to test state transitions.
 */
export function usePauseResume({ sessionId }: UsePauseResumeOptions): PauseResumeState {
  const toast = useToast()
  const qc = useQueryClient()

  const [isSuspended, setIsSuspended] = useState(false)
  const [pauseVotes, setPauseVotes] = useState<string[]>([])
  const [pauseRequired, setPauseRequired] = useState(0)
  const [resumeVotes, setResumeVotes] = useState<string[]>([])
  const [resumeRequired, setResumeRequired] = useState(0)
  const [votedPause, setVotedPause] = useState(false)
  const [votedResume, setVotedResume] = useState(false)

  const pauseMutation = useMutation({
    mutationFn: () => sessions.pause(sessionId),
    onSuccess: res => {
      setVotedPause(true)
      if (res.all_voted) {
        setIsSuspended(true)
        setPauseVotes([])
      }
      // Vote list is updated via WS pause_vote_update event.
    },
    onError: e => toast.showError(catchToAppError(e)),
  })

  const resumeMutation = useMutation({
    mutationFn: () => sessions.resume(sessionId),
    onSuccess: res => {
      setVotedResume(true)
      if (res.all_voted) {
        setIsSuspended(false)
        setResumeVotes([])
        setVotedPause(false)
        setVotedResume(false)
        qc.invalidateQueries({ queryKey: keys.session(sessionId) })
      }
      // Vote list is updated via WS resume_vote_update event.
    },
    onError: e => toast.showError(catchToAppError(e)),
  })

  return {
    isSuspended,
    pauseVotes,
    pauseRequired,
    resumeVotes,
    resumeRequired,
    votedPause,
    votedResume,
    isPausePending: pauseMutation.isPending,
    isResumePending: resumeMutation.isPending,

    // Setters for external sync (initial fetch + WS events)
    setSuspended: setIsSuspended,
    setPauseVoteUpdate: (votes, required) => {
      setPauseVotes(votes)
      setPauseRequired(required)
    },
    setResumeVoteUpdate: (votes, required) => {
      setResumeVotes(votes)
      setResumeRequired(required)
    },
    onSessionSuspended: () => {
      setIsSuspended(true)
      setPauseVotes([])
    },
    onSessionResumed: () => {
      setIsSuspended(false)
      setResumeVotes([])
      setVotedPause(false)
      setVotedResume(false)
      qc.invalidateQueries({ queryKey: keys.session(sessionId) })
    },

    // Mutations
    votePause: () => pauseMutation.mutate(),
    voteResume: () => resumeMutation.mutate(),
  }
}
