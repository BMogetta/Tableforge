import { request, validatedRequest } from '../api'
import type {
  ApplyMoveRequest,
  SurrenderRequest,
  VoteRematchRequest,
  VotePauseRequest,
  VoteResumeRequest,
} from '../schema-generated.zod'
import {
  moveSchema,
  applyMoveResponseSchema,
  pauseVoteResultSchema,
  readyVoteResultSchema,
  sessionEventSchema,
  getSessionResponseSchema,
  voteRematchResponseSchema,
} from '../schema-generated.zod'
import { z } from 'zod'

export const sessions = {
  get: (id: string) => validatedRequest(getSessionResponseSchema, `/sessions/${id}`),
  ready: (sessionId: string, playerId: string) =>
    validatedRequest(readyVoteResultSchema, `/sessions/${sessionId}/ready`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  move: (sessionId: string, playerId: string, payload: Record<string, unknown>) => {
    const body: ApplyMoveRequest = { player_id: playerId, payload }
    return validatedRequest(applyMoveResponseSchema, `/sessions/${sessionId}/move`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  surrender: (sessionId: string, playerId: string) => {
    const body: SurrenderRequest = { player_id: playerId }
    return validatedRequest(applyMoveResponseSchema, `/sessions/${sessionId}/surrender`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  rematch: (sessionId: string, playerId: string) => {
    const body: VoteRematchRequest = { player_id: playerId }
    return validatedRequest(voteRematchResponseSchema, `/sessions/${sessionId}/rematch`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  pause: (sessionId: string, playerId: string) => {
    const body: VotePauseRequest = { player_id: playerId }
    return validatedRequest(pauseVoteResultSchema, `/sessions/${sessionId}/pause`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  resume: (sessionId: string, playerId: string) => {
    const body: VoteResumeRequest = { player_id: playerId }
    return validatedRequest(pauseVoteResultSchema, `/sessions/${sessionId}/resume`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  // Manager only. Broadcasts room_closed to all clients.
  forceClose: (sessionId: string) => request<void>(`/sessions/${sessionId}`, { method: 'DELETE' }),
  events: (id: string) => validatedRequest(z.array(sessionEventSchema), `/sessions/${id}/events`),
  history: (id: string) => validatedRequest(z.array(moveSchema), `/sessions/${id}/history`),
}
