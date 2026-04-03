import { request } from '../api'
import type {
  Move,
  ApplyMoveRequest,
  ApplyMoveResponse,
  PauseVoteResult,
  ReadyVoteResult,
  SurrenderRequest,
  VoteRematchRequest,
  VoteRematchResponse,
  VotePauseRequest,
  VoteResumeRequest,
  SessionEvent,
  GetSessionResponse,
} from '../schema-generated'

export const sessions = {
  get: (id: string) => request<GetSessionResponse>(`/sessions/${id}`),
  ready: (sessionId: string, playerId: string) =>
    request<ReadyVoteResult>(`/sessions/${sessionId}/ready`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  move: (sessionId: string, playerId: string, payload: Record<string, unknown>) => {
    const body: ApplyMoveRequest = { player_id: playerId, payload }
    return request<ApplyMoveResponse>(`/sessions/${sessionId}/move`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  surrender: (sessionId: string, playerId: string) => {
    const body: SurrenderRequest = { player_id: playerId }
    return request<ApplyMoveResponse>(`/sessions/${sessionId}/surrender`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  rematch: (sessionId: string, playerId: string) => {
    const body: VoteRematchRequest = { player_id: playerId }
    return request<VoteRematchResponse>(`/sessions/${sessionId}/rematch`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  pause: (sessionId: string, playerId: string) => {
    const body: VotePauseRequest = { player_id: playerId }
    return request<PauseVoteResult>(`/sessions/${sessionId}/pause`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  resume: (sessionId: string, playerId: string) => {
    const body: VoteResumeRequest = { player_id: playerId }
    return request<PauseVoteResult>(`/sessions/${sessionId}/resume`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  // Manager only. Broadcasts room_closed to all clients.
  forceClose: (sessionId: string) => request<void>(`/sessions/${sessionId}`, { method: 'DELETE' }),
  events: (id: string) => request<SessionEvent[]>(`/sessions/${id}/events`),
  history: (id: string) => request<Move[]>(`/sessions/${id}/history`),
}
