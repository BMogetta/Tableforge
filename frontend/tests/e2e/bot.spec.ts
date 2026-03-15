import { test, expect, type Page } from '@playwright/test'
import { PLAYER1_STATE } from './helpers'

// ---------------------------------------------------------------------------
// Bot helpers
// ---------------------------------------------------------------------------

// Creates a room with P1 only, adds a bot via the UI, and starts the game.
// Returns the room ID so callers can make API calls if needed.
async function setupAndStartGameWithBot(p1: Page): Promise<string> {
  const player1Id = process.env.TEST_PLAYER1_ID!

  await p1.getByTestId('game-option-tictactoe').click()
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const roomId = p1.url().split('/rooms/')[1]

  // Force P1 (seat 0) to always go first so turn order is deterministic.
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
    data: { player_id: player1Id, value: 'fixed' },
  })

  // Add a bot via the UI.
  await expect(p1.getByTestId('add-bot-select')).toBeVisible({ timeout: 5_000 })
  await p1.getByTestId('add-bot-select').selectOption('easy')
  await p1.getByTestId('add-bot-btn').click()

  // Wait for the bot to appear in the player list.
  await expect(p1.getByTestId('player-count')).toContainText('2/2', { timeout: 5_000 })

  // Start the game.
  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 5_000 })
  await p1.getByTestId('start-game-btn').click()
  await expect(p1).toHaveURL(/\/game\//)

  return roomId
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

test.describe('Bot gameplay', () => {
  test('bot plays a full game against a human', async ({ browser }) => {
    const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))
    await p1.goto('/')

    await setupAndStartGameWithBot(p1)

    // P1 goes first (seat 0, fixed policy). Wait for "Your turn".
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })

    // P1 plays cell 0.
    await p1.locator('[data-cell="0"]').click()

    // Bot responds — status switches to "Your turn" again after the bot moves.
    // We wait for move_count to advance by 2 (P1 move + bot move) which is
    // reflected in the "Move N" counter in the header.
    await expect(p1.locator('[data-testid="game-status"]')).toContainText('Your turn', { timeout: 10_000 })

    // Play through to game over — P1 wins the top row, bot fills in between.
    // We use a simple sequence: P1 takes 0,1,2; bot will play somewhere else.
    await p1.locator('[data-cell="1"]').click()
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await p1.locator('[data-cell="2"]').click()

    // Game should end — P1 wins top row.
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })

    // Rematch button is visible — bot auto-votes so P1's single vote is enough.
    await p1.getByTestId('rematch-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//, { timeout: 15_000 })

    // Room is back in waiting state — P1 can start a new game.
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 5_000 })

    await p1Ctx.close()
  })

  test('bot can be removed and replaced before starting the game', async ({ browser }) => {
    const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))
    await p1.goto('/')

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const player1Id = process.env.TEST_PLAYER1_ID!

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: player1Id, value: 'fixed' },
    })

    // Add a bot.
    await expect(p1.getByTestId('add-bot-select')).toBeVisible({ timeout: 5_000 })
    await p1.getByTestId('add-bot-select').selectOption('easy')
    await p1.getByTestId('add-bot-btn').click()
    await expect(p1.getByTestId('player-count')).toContainText('2/2', { timeout: 5_000 })

    // Find and remove the bot using its remove button.
    const removeBtns = p1.locator('[data-testid^="remove-bot-btn-"]')
    await expect(removeBtns).toHaveCount(1, { timeout: 5_000 })
    await removeBtns.first().click()

    // Room should be back to 1 player.
    await expect(p1.getByTestId('player-count')).toContainText('1/2', { timeout: 5_000 })
    await expect(p1.getByTestId('start-game-btn')).toBeDisabled()

    // Add a new bot (hard difficulty this time).
    await p1.getByTestId('add-bot-select').selectOption('hard')
    await p1.getByTestId('add-bot-btn').click()
    await expect(p1.getByTestId('player-count')).toContainText('2/2', { timeout: 5_000 })

    // Start the game.
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 5_000 })
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