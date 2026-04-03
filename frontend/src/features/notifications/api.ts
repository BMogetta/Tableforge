import { request, validatedRequest } from '@/lib/api'
import { notificationSchema } from '@/lib/schema-generated.zod'
import { z } from 'zod'

export const notifications = {
  list: (playerId: string, includeRead = false) =>
    validatedRequest(
      z.array(notificationSchema),
      `/players/${playerId}/notifications?include_read=${includeRead}&player_id=${playerId}`,
    ),
  markRead: (playerId: string, notificationId: string) =>
    request<void>(`/notifications/${notificationId}/read`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  accept: (playerId: string, notificationId: string) =>
    request<void>(`/notifications/${notificationId}/accept`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
  decline: (playerId: string, notificationId: string) =>
    request<void>(`/notifications/${notificationId}/decline`, {
      method: 'POST',
      body: JSON.stringify({ player_id: playerId }),
    }),
}
