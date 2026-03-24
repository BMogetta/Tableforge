import { useState } from 'react'
import { useMutation } from '@tanstack/react-query'
import { useNavigate } from '@tanstack/react-router'
import { sessions } from '@/lib/api/sessions'
import { catchToAppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'

interface UseRematchOptions {
  sessionId: string
  playerId: string
}

export interface RematchState {
  rematchVotes: number
  totalPlayers: number
  votedRematch: boolean
  isPending: boolean
  voteRematch: () => void
  /** Called by useGameSocket when rematch_vote WS event is received. */
  onRematchVote: (votes: number, total: number) => void
  /** Called by useGameSocket when rematch_ready WS event is received. */
  onRematchReady: (roomId: string) => void
}

/**
 * Manages rematch voting state and navigation for a finished game session.
 *
 * Responsibilities:
 * - Tracks local voted state to prevent double-voting and update button UI.
 * - Navigates to the room when all players have voted (rematch_ready WS event).
 * - Exposes setters for useGameSocket to update vote counts from WS events.
 *
 * @testability
 * Mock `sessions.rematch` from `../../lib/api`.
 * Mock `useNavigate` from `@tanstack/react-router`.
 * Use renderHook to test that voteRematch calls the API and updates state.
 */
export function useRematch({ sessionId, playerId }: UseRematchOptions): RematchState {
  const toast = useToast()
  const navigate = useNavigate()

  const [rematchVotes, setRematchVotes] = useState(0)
  const [totalPlayers, setTotalPlayers] = useState(0)
  const [votedRematch, setVotedRematch] = useState(false)

  const mutation = useMutation({
    mutationFn: () => sessions.rematch(sessionId, playerId),
    onSuccess: res => {
      setVotedRematch(true)
      setRematchVotes(res.votes)
      setTotalPlayers(res.total_players)
    },
    onError: e => toast.showError(catchToAppError(e)),
  })

  return {
    rematchVotes,
    totalPlayers,
    votedRematch,
    isPending: mutation.isPending,
    voteRematch: () => mutation.mutate(),
    onRematchVote: (votes, total) => {
      setRematchVotes(votes)
      setTotalPlayers(total)
    },
    onRematchReady: (roomId: string) => {
      navigate({ to: '/rooms/$roomId', params: { roomId } })
    },
  }
}
