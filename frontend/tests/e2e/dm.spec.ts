import { test, expect } from './fixtures'

test.describe('Direct Messages', () => {
  test('P1 sends DM to P2 via API, P2 sees it in conversation', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    // Send DM via API.
    const res = await p1.request.post(`/api/v1/players/${p2Id}/dm`, {
      data: { player_id: p1Id, content: 'hello from e2e' },
    })
    expect(res.ok()).toBe(true)

    // P2 opens DM inbox.
    await p2.getByTestId('dm-envelope-btn').click()
    await expect(p2.getByTestId('dm-inbox-panel')).toBeVisible()

    // P2 should see a conversation with P1.
    await expect(p2.getByTestId(`dm-conversation-${p1Id}`)).toBeVisible({ timeout: 10_000 })
    await p2.getByTestId(`dm-conversation-${p1Id}`).click()

    // The message should be visible in the conversation.
    await expect(p2.locator('text=hello from e2e')).toBeVisible({ timeout: 10_000 })
  })

  test('P2 replies, P1 sees both messages', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    // P1 sends initial DM via API.
    await p1.request.post(`/api/v1/players/${p2Id}/dm`, {
      data: { player_id: p1Id, content: 'first message' },
    })

    // P2 opens conversation and replies via UI.
    await p2.getByTestId('dm-envelope-btn').click()
    await expect(p2.getByTestId(`dm-conversation-${p1Id}`)).toBeVisible({ timeout: 10_000 })
    await p2.getByTestId(`dm-conversation-${p1Id}`).click()

    await p2.getByTestId('dm-input').fill('reply from p2')
    await p2.getByTestId('dm-send-btn').click()

    // Both messages should be visible for P2.
    await expect(p2.locator('text=first message')).toBeVisible({ timeout: 10_000 })
    await expect(p2.locator('text=reply from p2')).toBeVisible({ timeout: 10_000 })

    // P1 opens conversation — should see both messages.
    await p1.getByTestId('dm-envelope-btn').click()
    await expect(p1.getByTestId(`dm-conversation-${p2Id}`)).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId(`dm-conversation-${p2Id}`).click()

    await expect(p1.locator('text=first message')).toBeVisible({ timeout: 10_000 })
    await expect(p1.locator('text=reply from p2')).toBeVisible({ timeout: 10_000 })
  })

  test('DM received via WebSocket shows in real-time', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    // P1 sends initial message so P2 has a conversation open.
    await p1.request.post(`/api/v1/players/${p2Id}/dm`, {
      data: { player_id: p1Id, content: 'setup message' },
    })

    // P2 opens the conversation.
    await p2.getByTestId('dm-envelope-btn').click()
    await expect(p2.getByTestId(`dm-conversation-${p1Id}`)).toBeVisible({ timeout: 10_000 })
    await p2.getByTestId(`dm-conversation-${p1Id}`).click()
    await expect(p2.locator('text=setup message')).toBeVisible({ timeout: 10_000 })

    // P1 sends another message while P2's conversation is open.
    await p1.request.post(`/api/v1/players/${p2Id}/dm`, {
      data: { player_id: p1Id, content: 'live ws message' },
    })

    // P2 should see it appear in real-time without refreshing.
    // Longer timeout — WS may need time to deliver after rebuild.
    await expect(p2.locator('text=live ws message')).toBeVisible({ timeout: 15_000 })
  })

  test('unread DM badge shows count on envelope button', async ({ players }) => {
    const { p1, p2, p1Id, p2Id } = players

    // P1 sends DM to P2.
    await p1.request.post(`/api/v1/players/${p2Id}/dm`, {
      data: { player_id: p1Id, content: 'unread test' },
    })

    // P2 reloads to pick up unread count.
    await p2.reload()
    await expect(p2.getByTestId('dm-envelope-btn')).toBeVisible({ timeout: 10_000 })

    // Badge should show a number.
    await expect.poll(
      async () => {
        const text = await p2.getByTestId('dm-envelope-btn').textContent()
        return text !== null && /\d/.test(text)
      },
      { timeout: 10_000, message: 'DM badge should show unread count' },
    ).toBe(true)
  })
})
