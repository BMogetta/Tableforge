import { request } from '@/lib/api'
import type { AllowedEmail, Player, PlayerRole } from '@/lib/api'

export const admin = {
  listEmails: () => request<AllowedEmail[]>('/admin/allowed-emails'),
  addEmail: (email: string, role: PlayerRole = 'player') =>
    request<AllowedEmail>('/admin/allowed-emails', {
      method: 'POST',
      body: JSON.stringify({ email, role }),
    }),
  removeEmail: (email: string) =>
    request<void>(`/admin/allowed-emails/${encodeURIComponent(email)}`, {
      method: 'DELETE',
    }),
  listPlayers: () => request<Player[]>('/admin/players'),
  setRole: (playerId: string, role: PlayerRole) =>
    request<void>(`/admin/players/${playerId}/role`, {
      method: 'PUT',
      body: JSON.stringify({ role }),
    }),
}
