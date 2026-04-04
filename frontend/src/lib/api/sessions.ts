import { request, validatedRequest } from '../api'
import type {
  ApplyMoveRequest,
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
  ready: (sessionId: string) =>
    validatedRequest(readyVoteResultSchema, `/sessions/${sessionId}/ready`, {
      method: 'POST',
    }),
  move: (sessionId: string, payload: Record<string, unknown>) => {
    const body: ApplyMoveRequest = { payload }
    return validatedRequest(applyMoveResponseSchema, `/sessions/${sessionId}/move`, {
      method: 'POST',
      body: JSON.stringify(body),
    })
  },
  surrender: (sessionId: string) => {
    return validatedRequest(applyMoveResponseSchema, `/sessions/${sessionId}/surrender`, {
      method: 'POST',
    })
  },
  rematch: (sessionId: string) => {
    return validatedRequest(voteRematchResponseSchema, `/sessions/${sessionId}/rematch`, {
      method: 'POST',
    })
  },
  pause: (sessionId: string) => {
    return validatedRequest(pauseVoteResultSchema, `/sessions/${sessionId}/pause`, {
      method: 'POST',
    })
  },
  resume: (sessionId: string) => {
    return validatedRequest(pauseVoteResultSchema, `/sessions/${sessionId}/resume`, {
      method: 'POST',
    })
  },
  // Manager only. Broadcasts room_closed to all clients.
  forceClose: (sessionId: string) => request<void>(`/sessions/${sessionId}`, { method: 'DELETE' }),
  events: (id: string) => validatedRequest(z.array(sessionEventSchema), `/sessions/${id}/events`),
  history: (id: string) => validatedRequest(z.array(moveSchema), `/sessions/${id}/history`),
}
