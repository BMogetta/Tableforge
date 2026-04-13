import { request, validatedRequest } from '../api'
import type { ApplyMoveRequest } from '../schema-generated.zod'
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
      body: JSON.stringify({}),
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
      body: JSON.stringify({}),
    })
  },
  rematch: (sessionId: string) => {
    return validatedRequest(voteRematchResponseSchema, `/sessions/${sessionId}/rematch`, {
      method: 'POST',
      body: JSON.stringify({}),
    })
  },
  pause: (sessionId: string) => {
    return validatedRequest(pauseVoteResultSchema, `/sessions/${sessionId}/pause`, {
      method: 'POST',
      body: JSON.stringify({}),
    })
  },
  resume: (sessionId: string) => {
    return validatedRequest(pauseVoteResultSchema, `/sessions/${sessionId}/resume`, {
      method: 'POST',
      body: JSON.stringify({}),
    })
  },
  // Manager only. Broadcasts room_closed to all clients.
  forceClose: (sessionId: string) =>
    request<void>(`/sessions/${sessionId}`, { method: 'DELETE', body: JSON.stringify({}) }),
  events: (id: string) => validatedRequest(z.array(sessionEventSchema), `/sessions/${id}/events`),
  history: (id: string) => validatedRequest(z.array(moveSchema), `/sessions/${id}/history`),

  // Debug-only: only registered on the server when ALLOW_DEBUG_SCENARIOS=true
  // (auto-true under TEST_MODE). Used by the dev ScenarioPicker.
  loadScenario: (sessionId: string, scenarioId: string) =>
    request<unknown>(`/sessions/${sessionId}/debug/load-scenario`, {
      method: 'POST',
      body: JSON.stringify({ scenario_id: scenarioId }),
    }),
}

export interface ScenarioSummary {
  id: string
  name: string
  description: string
}

export const scenarios = {
  list: (gameId: string) => request<ScenarioSummary[]>(`/games/${gameId}/scenarios`),
}
