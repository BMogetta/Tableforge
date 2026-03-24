import { request } from '../api'
import type {
  MoveDTO,
  MoveResponse,
  PauseVoteResult,
  ReadyVoteResult,
  RematchResponse,
  SessionEventDTO,
  SessionResponse,
} from '../api-generated'

export const sessions = {
  get: (id: string) => request<SessionResponse>(`/sessions/${id}`),
  ready: (sessionId: string, playerId: string) =>
    request<ReadyVoteResult>(`/sessions/${sessionId}/ready`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  move: (sessionId: string, playerId: string, payload: unknown) =>
    request<MoveResponse>(`/sessions/${sessionId}/move`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId, payload }),
    }),
  surrender: (sessionId: string, playerId: string) =>
    request<MoveResponse>(`/sessions/${sessionId}/surrender`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  rematch: (sessionId: string, playerId: string) =>
    request<RematchResponse>(`/sessions/${sessionId}/rematch`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  pause: (sessionId: string, playerId: string) =>
    request<PauseVoteResult>(`/sessions/${sessionId}/pause`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  resume: (sessionId: string, playerId: string) =>
    request<PauseVoteResult>(`/sessions/${sessionId}/resume`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  // Manager only. Broadcasts room_closed to all clients.
  forceClose: (sessionId: string) => request<void>(`/sessions/${sessionId}`, { method: 'DELETE' }),
  events: (id: string) => request<SessionEventDTO[]>(`/sessions/${id}/events`),
  history: (id: string) => request<MoveDTO[]>(`/sessions/${id}/history`),
}
