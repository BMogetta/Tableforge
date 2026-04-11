import { test, expect } from './fixtures'
import type { Page } from '@playwright/test'

/** Remove friendship between two players (idempotent). */
async function cleanFriendship(page: Page, playerId: string, otherId: string) {
  await page.request.delete(`/api/v1/players/${playerId}/friends/${otherId}`).catch(() => {})
  await page.request.delete(`/api/v1/players/${playerId}/friends/${otherId}/decline`).catch(() => {})
}

test.describe('Friends', () => {
  test.describe.configure({ mode: 'serial' })

  test('friend request appears in pending list', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    await cleanFriendship(p1, p1Id, p2Id)
    await cleanFriendship(p2, p2Id, p1Id)

    // Send request via API.
    await p1.request.post(`/api/v1/players/${p1Id}/friends/${p2Id}`)

    // P2 opens friends panel, switches to pending tab.
    await p2.getByTestId('friends-btn').click()
    await expect(p2.getByTestId('friends-panel')).toBeVisible()
    await p2.getByTestId('pending-tab').click()

    // P2 should see the pending request from P1.
    await expect(p2.getByTestId(`pending-${p1Id}`)).toBeVisible({ timeout: 10_000 })
  })

  test('accepting friend request shows both as friends', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    await cleanFriendship(p1, p1Id, p2Id)
    await cleanFriendship(p2, p2Id, p1Id)

    // Send request via API.
    await p1.request.post(`/api/v1/players/${p1Id}/friends/${p2Id}`)

    // P2 opens friends, goes to pending, accepts.
    await p2.getByTestId('friends-btn').click()
    await p2.getByTestId('pending-tab').click()
    await expect(p2.getByTestId(`pending-${p1Id}`)).toBeVisible({ timeout: 10_000 })
    await p2.getByTestId('accept-btn').click()

    // P2 switches to friends tab — P1 should be there.
    await p2.getByTestId('friends-tab').click()
    await expect(p2.getByTestId(`friend-${p1Id}`)).toBeVisible({ timeout: 10_000 })

    // P1 opens friends — P2 should be there.
    await p1.getByTestId('friends-btn').click()
    await expect(p1.getByTestId(`friend-${p2Id}`)).toBeVisible({ timeout: 10_000 })
  })

  test('declining friend request removes it from pending', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    await cleanFriendship(p1, p1Id, p2Id)
    await cleanFriendship(p2, p2Id, p1Id)

    // P2 sends request to P1 via API.
    await p2.request.post(`/api/v1/players/${p2Id}/friends/${p1Id}`)

    // P1 opens pending, declines.
    await p1.getByTestId('friends-btn').click()
    await p1.getByTestId('pending-tab').click()
    await expect(p1.getByTestId(`pending-${p2Id}`)).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('decline-btn').click()

    // Pending request should disappear.
    await expect(p1.getByTestId(`pending-${p2Id}`)).not.toBeVisible({ timeout: 5_000 })
  })

  test('removing a friend removes them from the list', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    await cleanFriendship(p1, p1Id, p2Id)
    await cleanFriendship(p2, p2Id, p1Id)

    // Become friends via API.
    await p1.request.post(`/api/v1/players/${p1Id}/friends/${p2Id}`)
    await p2.request.put(`/api/v1/players/${p2Id}/friends/${p1Id}/accept`)

    // P1 opens friends, removes P2.
    await p1.getByTestId('friends-btn').click()
    await expect(p1.getByTestId(`friend-${p2Id}`)).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('remove-btn').click()

    // P2 should disappear from P1's list.
    await expect(p1.getByTestId(`friend-${p2Id}`)).not.toBeVisible({ timeout: 5_000 })
  })
})
