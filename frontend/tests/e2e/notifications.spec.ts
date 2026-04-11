import { test, expect } from './fixtures'
import type { Page } from '@playwright/test'

/** Remove friendship between two players (idempotent). */
async function cleanFriendship(page: Page, playerId: string, otherId: string) {
  await page.request.delete(`/api/v1/players/${playerId}/friends/${otherId}`).catch(() => {})
  await page.request.delete(`/api/v1/players/${playerId}/friends/${otherId}/decline`).catch(() => {})
}

interface Notification {
  id: string
  type: string
  action_taken: string | null
}

test.describe('Notifications', () => {
  test('friend request creates notification for addressee', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    await cleanFriendship(p1, p1Id, p2Id)
    await cleanFriendship(p2, p2Id, p1Id)

    // P1 sends friend request to P2 via API.
    const res = await p1.request.post(`/api/v1/players/${p1Id}/friends/${p2Id}`)
    expect(res.ok()).toBe(true)

    // Verify notification was created via API (include_read=true to see all).
    await expect.poll(
      async () => {
        const r = await p2.request.get(
          `/api/v1/players/${p2Id}/notifications?include_read=true&limit=50&offset=0`,
        )
        if (!r.ok()) return false
        const data = await r.json()
        return data.items.some(
          (n: Notification) => n.type === 'friend_request' && n.action_taken == null,
        )
      },
      { timeout: 10_000, message: 'P2 should have a friend_request notification' },
    ).toBe(true)
  })

  test('accepting notification via API creates friendship', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    await cleanFriendship(p1, p1Id, p2Id)
    await cleanFriendship(p2, p2Id, p1Id)

    // Send request via API.
    await p1.request.post(`/api/v1/players/${p1Id}/friends/${p2Id}`)

    // Poll for the notification (service may need a moment after rebuild).
    let friendReqNotif: { id: string } | undefined
    await expect.poll(
      async () => {
        const r = await p2.request.get(
          `/api/v1/players/${p2Id}/notifications?include_read=true&limit=50&offset=0`,
        )
        if (!r.ok()) return false
        const d = await r.json()
        friendReqNotif = d.items.find(
          (n: Notification) => n.type === 'friend_request' && n.action_taken == null,
        )
        return !!friendReqNotif
      },
      { timeout: 10_000, message: 'P2 should have a friend_request notification' },
    ).toBe(true)

    const acceptRes = await p2.request.post(`/api/v1/notifications/${friendReqNotif.id}/accept`)
    expect(acceptRes.ok()).toBe(true)

    // Both should now be friends.
    await expect.poll(
      async () => {
        const r = await p1.request.get(`/api/v1/players/${p1Id}/friends`)
        if (!r.ok()) return false
        const friends = await r.json()
        if (!Array.isArray(friends)) return false
        return friends.some((f: { friend_id: string }) => f.friend_id === p2Id)
      },
      { timeout: 10_000, message: 'P1 should see P2 as friend after accept' },
    ).toBe(true)
  })

  test('notification badge shows unread count', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    await cleanFriendship(p1, p1Id, p2Id)
    await cleanFriendship(p2, p2Id, p1Id)

    // Send request to create a notification.
    await p1.request.post(`/api/v1/players/${p1Id}/friends/${p2Id}`)

    // P2's notification badge should show a count > 0.
    // Reload to pick up the new notification.
    await p2.reload()
    await expect(p2.getByTestId('notifications-btn')).toBeVisible({ timeout: 10_000 })

    // The badge text should contain a number (e.g., "1", "2", "9+").
    await expect.poll(
      async () => {
        const text = await p2.getByTestId('notifications-btn').textContent()
        return text !== null && /\d/.test(text)
      },
      { timeout: 10_000, message: 'Notification badge should show a count' },
    ).toBe(true)
  })
})
