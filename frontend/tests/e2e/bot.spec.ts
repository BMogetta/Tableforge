import { test, expect, type Page } from '@playwright/test'
import { getPair, patchEmptyBodyRoutes, type PlayerPair } from './helpers'

// ---------------------------------------------------------------------------
// Bot helpers
// ---------------------------------------------------------------------------

// Creates a room with P1 only, adds a bot via the UI, and starts the game.
// Returns the room ID so callers can make API calls if needed.
async function setupAndStartGameWithBot(p1: Page, p1Id: string): Promise<string> {
  await p1.getByTestId('game-option-tictactoe').click()
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const roomId = p1.url().split('/rooms/')[1]

  // Force P1 (seat 0) to always go first so turn order is deterministic.
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
    data: { player_id: p1Id, value: 'fixed' },
  })

  // Open bot toolbar to add a bot via the UI.
  await p1.getByTestId('toolbar-bot').click()
  await expect(p1.getByTestId('add-bot-select')).toBeVisible({ timeout: 5000 })
  await p1.getByTestId('add-bot-select').selectOption('easy')
  await p1.getByTestId('add-bot-btn').click()

  // Wait for the bot to appear in the player list.
  await expect(p1.getByTestId('player-count')).toContainText('2/2', { timeout: 5000 })

  // Close bot popover if still open (room is now full so the button gets disabled).
  const backdrop = p1.locator('[class*="popoverBackdrop"]')
  if (await backdrop.isVisible().catch(() => false)) {
    await backdrop.click({ position: { x: 5, y: 5 } })
  }

  // Start the game.
  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 5000 })
  await p1.getByTestId('start-game-btn').click()
  await expect(p1).toHaveURL(/\/game\//)

  return roomId
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Bot gameplay', () => {
  test('bot plays a full game against a human', async ({ browser }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const p1Ctx = await browser.newContext({ storageState: pair.p1State })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))
    await patchEmptyBodyRoutes(p1)
    await p1.goto('/')

    await setupAndStartGameWithBot(p1, pair.p1Id)

    // P1 goes first (seat 0, fixed policy). Wait for "Your turn".
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })

    // P1 plays cell 0.
    await p1.locator('[data-cell="0"]').click()

    // Bot responds — status switches to "Your turn" again after the bot moves.
    await expect(p1.locator('[data-testid="game-status"]')).toContainText('Your turn', {
      timeout: 10_000,
    })

    // Play random available cells until the game ends — outcome doesn't matter.
    const status = p1.getByTestId('game-status')

    while (true) {
      const isOver = await status.filter({ hasText: /You won|You lost|Draw/ }).count()
      if (isOver) break

      await expect(status).toContainText(/Your turn|You won|You lost|Draw/, { timeout: 10_000 })

      const isOverNow = await status.filter({ hasText: /You won|You lost|Draw/ }).count()
      if (isOverNow) break

      const moveBefore = await p1.locator('text=/Move \\d+/').textContent()

      const cells = p1.locator('[data-cell]')
      const count = await cells.count()
      let moved = false
      for (let i = 0; i < count; i++) {
        const cell = cells.nth(i)
        const label = await cell.getAttribute('aria-label')
        const enabled = await cell.isEnabled()
        // Empty cell has aria-label "Cell N", occupied has "Cell N: X" or "Cell N: O"
        if (enabled && label && !label.includes(':')) {
          await cell.click()
          moved = true
          break
        }
      }

      if (!moved) {
        // No clickable cells found — wait for game to end.
        await expect(status).toContainText(/You won|You lost|Draw/, { timeout: 10_000 })
        break
      }

      // Wait for move to be processed — counter advances or game ends.
      await expect(p1.locator('text=/Move \\d+/')).not.toHaveText(moveBefore!, { timeout: 5000 })
    }

    // Game should be over — any outcome is valid.
    await expect(p1.getByTestId('game-status')).toContainText(/You won|You lost|Draw/, {
      timeout: 10_000,
    })

    // Rematch button is visible — bot auto-votes so the rematch resolves immediately.
    // With a bot, both votes arrive and a new game starts directly (no lobby).
    await p1.getByTestId('rematch-btn').click()

    // Either we land back in the room lobby or directly in a new game.
    await expect(p1).toHaveURL(/\/(rooms|game)\//, { timeout: 15_000 })

    await p1Ctx.close()
  })

  test('bot can be removed and replaced before starting the game', async ({
    browser,
  }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const p1Ctx = await browser.newContext({ storageState: pair.p1State })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))
    await patchEmptyBodyRoutes(p1)
    await p1.goto('/')

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: pair.p1Id, value: 'fixed' },
    })

    // Open bot toolbar and add a bot.
    await p1.getByTestId('toolbar-bot').click()
    await expect(p1.getByTestId('add-bot-select')).toBeVisible({ timeout: 5000 })
    await p1.getByTestId('add-bot-select').selectOption('easy')
    await p1.getByTestId('add-bot-btn').click()
    await expect(p1.getByTestId('player-count')).toContainText('2/2', { timeout: 5000 })

    // Find and remove the bot using its remove button.
    const removeBtns = p1.locator('[data-testid^="remove-bot-btn-"]')
    await expect(removeBtns).toHaveCount(1, { timeout: 5000 })
    await removeBtns.first().click()

    // Room should be back to 1 player.
    await expect(p1.getByTestId('player-count')).toContainText('1/2', { timeout: 5000 })
    await expect(p1.getByTestId('start-game-btn')).toBeDisabled()

    // Open bot toolbar and add a new bot (hard difficulty).
    await p1.getByTestId('toolbar-bot').click()
    await expect(p1.getByTestId('add-bot-select')).toBeVisible({ timeout: 5000 })
    await p1.getByTestId('add-bot-select').selectOption('hard')
    await p1.getByTestId('add-bot-btn').click()
    await expect(p1.getByTestId('player-count')).toContainText('2/2', { timeout: 5000 })

    // Start the game.
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 5000 })
    await p1.getByTestId('start-game-btn').click()
    await expect(p1).toHaveURL(/\/game\//)

    // Verify the game is running — P1 gets first turn.
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })

    // P1 makes a move — bot should respond.
    await p1.locator('[data-cell="4"]').click()
    // After bot moves, it's P1's turn again.
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
  })
})
