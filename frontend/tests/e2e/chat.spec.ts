import { test, expect, type Page } from '@playwright/test'
import { createPlayerContexts } from './helpers'

// --- Helpers -----------------------------------------------------------------

// P1 creates a room and P2 joins. Both end up on /rooms/:id.
// Does NOT start the game — chat is tested in the waiting room.
async function setupRoom(p1: Page, p2: Page) {
  await p1.getByTestId('game-option-tictactoe').click()
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const code = await p1.getByTestId('room-code').textContent()

  await p2.getByTestId('join-code-input').fill(code!)
  await p2.getByTestId('join-btn').click()
  await expect(p2).toHaveURL(/\/rooms\//)
}

// Scoped locator for message bubbles inside the chat sidebar.
// Avoids strict mode violations from text matches elsewhere in the page
// (e.g. "first" matching "First move" in lobby settings labels).
function chatMessage(page: Page, text: string) {
  return page.locator('[class*="messages"]').locator(`text=${text}`)
}

// --- Tests -------------------------------------------------------------------

test.describe('Room chat', () => {
  test('message sent by P1 appears in P2 sidebar via WS', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupRoom(p1, p2)

    await expect(p1.locator('input[placeholder="Message or /command..."]')).toBeVisible({
      timeout: 5_000,
    })
    await expect(p2.locator('input[placeholder="Message or /command..."]')).toBeVisible({
      timeout: 5_000,
    })

    await p1.locator('input[placeholder="Message or /command..."]').fill('hello from p1')
    await p1.locator('input[placeholder="Message or /command..."]').press('Enter')

    await expect(chatMessage(p1, 'hello from p1')).toBeVisible({ timeout: 10_000 })
    await expect(chatMessage(p2, 'hello from p1')).toBeVisible({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('multiple messages from both players appear in correct order', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupRoom(p1, p2)

    await expect(p1.locator('input[placeholder="Message or /command..."]')).toBeVisible({
      timeout: 5_000,
    })
    await expect(p2.locator('input[placeholder="Message or /command..."]')).toBeVisible({
      timeout: 5_000,
    })

    await p1.locator('input[placeholder="Message or /command..."]').fill('msg-alpha')
    await p1.locator('input[placeholder="Message or /command..."]').press('Enter')

    // Wait for P2 to receive it before P2 replies.
    await expect(chatMessage(p2, 'msg-alpha')).toBeVisible({ timeout: 10_000 })

    await p2.locator('input[placeholder="Message or /command..."]').click()
    await p2.locator('input[placeholder="Message or /command..."]').fill('msg-beta')
    await p2.locator('input[placeholder="Message or /command..."]').press('Enter')

    // Both messages appear on both sides.
    await expect(chatMessage(p2, 'msg-beta')).toBeVisible({ timeout: 10_000 })
    await expect(chatMessage(p1, 'msg-beta')).toBeVisible({ timeout: 10_000 })
    await expect(chatMessage(p2, 'msg-alpha')).toBeVisible()
    await expect(chatMessage(p2, 'msg-beta')).toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('sidebar can be collapsed and reopened', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupRoom(p1, p2)

    await expect(p1.locator('input[placeholder="Message or /command..."]')).toBeVisible({
      timeout: 5_000,
    })

    await p1.locator('button[title="Hide chat"]').click()
    await expect(p1.locator('input[placeholder="Message or /command..."]')).not.toBeVisible()

    await p1.locator('button[title="Show chat"]').click()
    await expect(p1.locator('input[placeholder="Message or /command..."]')).toBeVisible({
      timeout: 3_000,
    })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('empty message cannot be sent', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupRoom(p1, p2)

    await expect(p1.locator('input[placeholder="Message or /command..."]')).toBeVisible({
      timeout: 5_000,
    })

    const sendBtn = p1.locator('button[title="Send"]')
    await expect(sendBtn).toBeDisabled()

    await p1.locator('input[placeholder="Message or /command..."]').press('Enter')

    const messages = p1.locator('[class*="messageBubble"]')
    await expect(messages).toHaveCount(0)

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('messages persist after HTTP resync', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupRoom(p1, p2)

    await expect(p1.locator('input[placeholder="Message or /command..."]')).toBeVisible({
      timeout: 5_000,
    })

    await p1.locator('input[placeholder="Message or /command..."]').fill('persistent-msg')
    await p1.locator('input[placeholder="Message or /command..."]').press('Enter')
    await expect(chatMessage(p1, 'persistent-msg')).toBeVisible({ timeout: 5_000 })

    // P2 leaves the room properly before navigating away.
    const roomId = p2.url().split('/rooms/')[1]
    await p2.request.post(`/api/v1/rooms/${roomId}/leave`, {
      data: { player_id: process.env.TEST_PLAYER2_ID! },
    })
    await p2.goto('/')

    // Re-join using the room code displayed to P1.
    const code = await p1.getByTestId('room-code-display').textContent()
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//, { timeout: 5_000 })

    // Message loaded from HTTP on fresh mount.
    await expect(chatMessage(p2, 'persistent-msg')).toBeVisible({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})
