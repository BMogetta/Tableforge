import { request, validatedRequest } from '@/lib/api'
import { listNotificationsResponseSchema } from '@/lib/schema-generated.zod'

export const notifications = {
  list: (playerId: string, includeRead = false, limit = 20, offset = 0) =>
    validatedRequest(
      listNotificationsResponseSchema,
      `/players/${playerId}/notifications?include_read=${includeRead}&player_id=${playerId}&limit=${limit}&offset=${offset}`,
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
