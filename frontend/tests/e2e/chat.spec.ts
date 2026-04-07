import { test, expect } from './fixtures'
import type { Page } from '@playwright/test'
import { setupRoom } from './helpers'

const chatInput = 'input[placeholder="Message or /command..."]'

/** Open the chat popover for a page via the toolbar button. */
async function openChat(page: Page) {
  await page.getByTestId('toolbar-chat').click()
  await expect(page.locator(chatInput)).toBeVisible({ timeout: 5000 })
}

/** Close the chat popover by clicking outside it. */
async function closeChat(page: Page) {
  // Click far from the popover area (top-left corner) to hit the backdrop.
  await page.locator('[class*="popoverBackdrop"]').click({ position: { x: 5, y: 5 } })
  await expect(page.locator(chatInput)).not.toBeVisible({ timeout: 5000 })
}

// Scoped locator for message bubbles inside the chat popover.
function chatMessage(page: Page, text: string) {
  return page.locator('[class*="messages"]').locator(`text=${text}`)
}

// --- Tests -------------------------------------------------------------------

test.describe('Room chat', () => {
  test('message sent by P1 appears in P2 sidebar via WS', async ({ players }) => {
    const { p1, p2 } = players
    await setupRoom(p1, p2)

    await openChat(p1)
    await openChat(p2)

    await p1.locator(chatInput).fill('hello from p1')
    await p1.locator(chatInput).press('Enter')

    await expect(chatMessage(p1, 'hello from p1')).toBeVisible({ timeout: 10_000 })
    await expect(chatMessage(p2, 'hello from p1')).toBeVisible({ timeout: 10_000 })
  })

  test('multiple messages from both players appear in correct order', async ({ players }) => {
    const { p1, p2 } = players
    await setupRoom(p1, p2)

    await openChat(p1)
    await openChat(p2)

    await p1.locator(chatInput).fill('msg-alpha')
    await p1.locator(chatInput).press('Enter')

    // Wait for P2 to receive it before P2 replies.
    await expect(chatMessage(p2, 'msg-alpha')).toBeVisible({ timeout: 10_000 })

    await p2.locator(chatInput).click()
    await p2.locator(chatInput).fill('msg-beta')
    await p2.locator(chatInput).press('Enter')

    // Both messages appear on both sides.
    await expect(chatMessage(p2, 'msg-beta')).toBeVisible({ timeout: 10_000 })
    await expect(chatMessage(p1, 'msg-beta')).toBeVisible({ timeout: 10_000 })
    await expect(chatMessage(p2, 'msg-alpha')).toBeVisible()
    await expect(chatMessage(p2, 'msg-beta')).toBeVisible()
  })

  test('chat popover can be toggled open and closed', async ({ players }) => {
    const { p1, p2 } = players
    await setupRoom(p1, p2)

    await openChat(p1)
    await closeChat(p1)

    // Reopen to confirm it works a second time.
    await openChat(p1)
  })

  test('empty message cannot be sent', async ({ players }) => {
    const { p1, p2 } = players
    await setupRoom(p1, p2)

    await openChat(p1)

    const sendBtn = p1.locator('button[title="Send"]')
    await expect(sendBtn).toBeDisabled()

    await p1.locator(chatInput).press('Enter')

    const messages = p1.locator('[class*="messageBubble"]')
    await expect(messages).toHaveCount(0)
  })

  test('messages persist after HTTP resync', async ({ players }) => {
    const { p1, p2, p2Id } = players
    await setupRoom(p1, p2)

    await openChat(p1)

    await p1.locator(chatInput).fill('persistent-msg')
    await p1.locator(chatInput).press('Enter')
    await expect(chatMessage(p1, 'persistent-msg')).toBeVisible({ timeout: 5000 })

    // P2 leaves the room properly before navigating away.
    const roomId = p2.url().split('/rooms/')[1]
    await p2.request.post(`/api/v1/rooms/${roomId}/leave`, {
      data: { player_id: p2Id },
    })
    await p2.goto('/')

    // Re-join using the room code shown to P1 in the header.
    const code = await p1.getByTestId('room-code').textContent()
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//, { timeout: 5000 })

    // Open chat on P2 and verify the message was loaded from HTTP.
    await openChat(p2)
    await expect(chatMessage(p2, 'persistent-msg')).toBeVisible({ timeout: 10_000 })
  })
})
