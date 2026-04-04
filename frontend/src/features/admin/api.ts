import { request } from '@/lib/api'
import type { PlayerRole } from '@/lib/api'
import type {
  AllowedEmail,
  Ban,
  GameSession,
  Player,
  PlayerReport,
  Room,
} from '@/lib/schema-generated.zod'
import type { RoomView } from '@/lib/api'

export interface SystemStats {
  online_players: number
  active_rooms: number
  active_sessions: number
  total_players: number
  total_sessions_today: number
}

export interface AuditLog {
  id: string
  actor_id: string
  action: string
  target_type: string
  target_id: string
  details?: Record<string, unknown>
  created_at: string
}

export interface AuditLogFilter {
  action?: string
  target_type?: string
  from?: string
  to?: string
  limit?: number
  offset?: number
}

export const admin = {
  // Emails
  listEmails: () => request<AllowedEmail[]>('/admin/allowed-emails'),
  addEmail: (email: string, role: PlayerRole = 'player') =>
    request<AllowedEmail>('/admin/allowed-emails', {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }),
  removeEmail: (email: string) =>
    request<void>(`/admin/allowed-emails/${encodeURIComponent(email)}`, {
      method: 'DELETE',
      body: JSON.stringify({}),
    }),

  // Players
  listPlayers: () => request<Player[]>('/admin/players'),
  setRole: (playerId: string, role: PlayerRole) =>
    request<void>(`/admin/players/${playerId}/role`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),

  // Reports
  listReports: (status?: string) =>
    request<PlayerReport[]>(
      `/admin/reports${status && status !== 'all' ? `?status=${status}` : ''}`,
    ),
  listPlayerReports: (playerId: string) =>
    request<PlayerReport[]>(`/admin/players/${playerId}/reports`),
  reviewReport: (reportId: string, resolution: string, banId?: string) =>
    request<void>(`/admin/reports/${reportId}/review`, {
      method: 'PUT',
      body: JSON.stringify({ resolution, ban_id: banId }),
    }),

  // Bans
  banPlayer: (playerId: string, reason?: string, expiresAt?: string) =>
    request<Ban>(`/admin/players/${playerId}/ban`, {
      method: 'POST',
      body: JSON.stringify({ reason, expires_at: expiresAt }),
    }),
  liftBan: (banId: string) => request<void>(`/admin/bans/${banId}`, { method: 'DELETE' }),
  listPlayerBans: (playerId: string) => request<Ban[]>(`/admin/players/${playerId}/bans`),

  // Stats (served by game-server)
  getStats: () => request<SystemStats>('/admin/stats'),

  // Audit logs
  listAuditLogs: (filter?: AuditLogFilter) => {
    const params = new URLSearchParams()
    if (filter?.action) params.set('action', filter.action)
    if (filter?.target_type) params.set('target_type', filter.target_type)
    if (filter?.from) params.set('from', filter.from)
    if (filter?.to) params.set('to', filter.to)
    if (filter?.limit) params.set('limit', String(filter.limit))
    if (filter?.offset) params.set('offset', String(filter.offset))
    const qs = params.toString()
    return request<AuditLog[]>(`/admin/audit-logs${qs ? `?${qs}` : ''}`)
  },

  // Broadcast
  sendBroadcast: (message: string, type: 'info' | 'warning' = 'info') =>
    request<void>('/admin/broadcast', {
      method: 'POST',
      body: JSON.stringify({ message, type }),
    }),

  // Rooms
  listRooms: () => request<Room[]>('/rooms'),
  getRoom: (roomId: string) => request<RoomView>(`/rooms/${roomId}`),
  listPlayerSessions: (playerId: string) => request<GameSession[]>(`/players/${playerId}/sessions`),
  forceEndSession: (sessionId: string) =>
    request<void>(`/sessions/${sessionId}`, { method: 'DELETE' }),
}
