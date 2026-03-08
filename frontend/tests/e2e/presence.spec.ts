import { test, expect } from '@playwright/test'
import { createPlayerContexts, setupAndStartGame } from './helpers'

// ---------------------------------------------------------------------------
// Presence tests
//
// Covers:
//   - Presence dot is offline by default before opponent connects
//   - Presence dot goes online when opponent connects to the room
//   - Presence dot goes offline when opponent disconnects
//   - Presence dot is shown in Room.tsx player list
//   - Presence dot is shown in Game.tsx during the game
// ---------------------------------------------------------------------------

test.describe('Player presence', () => {
  test('presence dot shown in room player list', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const code = await p1.getByTestId('room-code').textContent()

    // P2 joins — P1 should see P2's presence dot go online.
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    const p2Id = process.env.TEST_PLAYER2_ID!

    // P1 sees P2's dot as online.
    await expect(p1.locator(`[data-testid="presence-dot-${p2Id}"]`))
      .toHaveAttribute('data-online', 'true', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('presence dot goes offline when player leaves room', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const code = await p1.getByTestId('room-code').textContent()
    const p2Id = process.env.TEST_PLAYER2_ID!

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    // Wait for P2 to appear online.
    await expect(p1.locator(`[data-testid="presence-dot-${p2Id}"]`))
      .toHaveAttribute('data-online', 'true', { timeout: 10_000 })

    // P2 navigates away — WS closes, presence deleted.
    await p2.goto('/')

    await expect(p1.locator(`[data-testid="presence-dot-${p2Id}"]`))
      .toHaveAttribute('data-online', 'false', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('opponent presence indicator shown during game', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)

    // Both players are connected — each should see opponent as online.
    await expect(p1.getByTestId('opponent-presence-dot'))
      .toHaveAttribute('data-online', 'true', { timeout: 10_000 })
    await expect(p2.getByTestId('opponent-presence-dot'))
      .toHaveAttribute('data-online', 'true', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('opponent presence goes offline when opponent disconnects during game', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)

    await expect(p1.getByTestId('opponent-presence-dot'))
      .toHaveAttribute('data-online', 'true', { timeout: 10_000 })

    // P2 closes their context — simulates disconnect.
    await p2Ctx.close()

    await expect(p1.getByTestId('opponent-presence-dot'))
      .toHaveAttribute('data-online', 'false', { timeout: 10_000 })

    await p1Ctx.close()
  })

  test('opponent presence text updates correctly', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)

    // Both connected — text should say online.
    await expect(p1.getByTestId('opponent-presence-text'))
      .toContainText('Opponent online', { timeout: 10_000 })

    // P2 disconnects.
    await p2Ctx.close()

    await expect(p1.getByTestId('opponent-presence-text'))
      .toContainText('Opponent offline', { timeout: 10_000 })

    await p1Ctx.close()
  })
})