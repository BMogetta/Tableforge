import { create } from 'zustand'

export const QueueStatus = {
  Idle: 'idle',
  Queued: 'queued',
  MatchFound: 'match_found',
} as const
export type QueueStatus = (typeof QueueStatus)[keyof typeof QueueStatus]

interface QueueState {
  queueStatus: QueueStatus
  queueJoinedAt: number | null
  matchId: string | null

  setQueued: (joinedAt: number) => void
  setMatchFound: (matchId: string) => void
  clearQueue: () => void
}

export const useQueueStore = create<QueueState>(set => ({
  queueStatus: QueueStatus.Idle,
  queueJoinedAt: null,
  matchId: null,

  setQueued: joinedAt =>
    set({ queueStatus: QueueStatus.Queued, queueJoinedAt: joinedAt, matchId: null }),
  setMatchFound: matchId => set({ queueStatus: QueueStatus.MatchFound, matchId }),
  clearQueue: () => set({ queueStatus: QueueStatus.Idle, queueJoinedAt: null, matchId: null }),
}))
