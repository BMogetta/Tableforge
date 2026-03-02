import { test, expect, Browser, BrowserContext, Page } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const PLAYER1_STATE = path.join(__dirname, '.auth/player1.json')
const PLAYER2_STATE = path.join(__dirname, '.auth/player2.json')

// --- Shared helpers ----------------------------------------------------------

// Creates two authenticated browser contexts and navigates both to the lobby.
async function createPlayerContexts(browser: Browser) {
  const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
  const p1 = await p1Ctx.newPage()
  p1.on('console', msg => console.log('P1:', msg.text()))
  p1.on('pageerror', err => console.log('P1 ERROR:', err.message))

  const p2Ctx = await browser.newContext({ storageState: PLAYER2_STATE })
  const p2 = await p2Ctx.newPage()
  p2.on('console', msg => console.log('P2:', msg.text()))
  p2.on('pageerror', err => console.log('P2 ERROR:', err.message))

  await p1.goto('/')
  await p2.goto('/')

  return { p1Ctx, p1, p2Ctx, p2 }
}

// P1 creates a room, P2 joins via the room code, P1 starts the game.
// Both pages are asserted to have navigated to /game/:id before returning.
async function setupAndStartGame(p1: Page, p2: Page) {
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const code = await p1.getByTestId('room-code').textContent()
  await p2.getByTestId('join-code-input').fill(code!)
  await p2.getByTestId('join-btn').click()
  await expect(p2).toHaveURL(/\/rooms\//)

  // Start button is disabled until the WS player_joined event updates the room.
  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
  await p1.getByTestId('start-game-btn').click()

  await expect(p1).toHaveURL(/\/game\//)
  await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })
}

// Plays a fixed winning sequence for TicTacToe.
// P1 wins the top row (cells 0, 1, 2). P2 plays cells 3 and 4.
// Each move waits for "Your turn" to confirm the server has advanced the turn.
async function playFullGame(p1: Page, p2: Page) {
  const moves = [
    { player: p1, cell: 0 },
    { player: p2, cell: 3 },
    { player: p1, cell: 1 },
    { player: p2, cell: 4 },
    { player: p1, cell: 2 },
  ]

  for (const { player, cell } of moves) {
    await expect(player.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await player.locator(`[data-cell="${cell}"]`).click()
  }
}

// --- Tests -------------------------------------------------------------------

test.describe('TicTacToe game', () => {
  test('two players can play a full game', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    // Assert final game-over state is correctly shown to both players.
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You lost', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('turn timeout ends the game', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)

    // Neither player moves. The server's turn timer fires after default_timeout_secs (30s)
    // and broadcasts game_over with the idle player as the loser.
    // Timeout is 35s to give the server a few seconds of margin.
    await expect(p1.getByTestId('game-status')).toContainText('You lost', { timeout: 35_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You won', { timeout: 35_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('player can forfeit a game', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)

    // Clicking ← Lobby mid-game should open a confirmation modal instead of
    // navigating immediately, to prevent accidental forfeits.
    await p1.locator('button', { hasText: '← Lobby' }).click()
    await expect(p1.getByRole('dialog')).toBeVisible()
    await expect(p1.getByRole('dialog')).toContainText('Forfeit game?')

    // Cancelling dismisses the modal and keeps the player in the game.
    await p1.getByRole('button', { name: 'Cancel' }).click()
    await expect(p1.getByRole('dialog')).not.toBeVisible()
    await expect(p1).toHaveURL(/\/game\//)

    // Confirming the forfeit ends the game: P1 is redirected to the lobby
    // and P2 receives game_over via WS with a win result.
    await p1.locator('button', { hasText: '← Lobby' }).click()
    await expect(p1.getByRole('dialog')).toBeVisible()
    await p1.getByTestId('confirm-surrender-btn').click()

    await expect(p1).toHaveURL('/', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('both players can rematch after a game ends', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    // Wait for game-over state to settle on both sides before interacting
    // with the rematch button — the WS game_over event must have been processed.
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You lost', { timeout: 10_000 })

    const p1GameUrl = p1.url()

    // P1 votes for rematch first. With only one vote in, the button should
    // switch to a waiting indicator and P2's button should remain active.
    await p1.getByTestId('rematch-btn').click()
    await expect(p1.getByTestId('rematch-btn')).toBeDisabled()
    await expect(p1.getByTestId('rematch-btn')).toContainText('Waiting')

    // P2 receives the rematch_vote WS event — their button stays enabled.
    await expect(p2.getByTestId('rematch-btn')).toBeEnabled()

    // P2 votes. Both players have now voted, so the server creates a new session
    // and broadcasts rematch_started — both pages should navigate to a new game URL.
    await p2.getByTestId('rematch-btn').click()

    await expect(p1).toHaveURL(/\/game\//, { timeout: 10_000 })
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // The new game URL must differ from the original session — a fresh session
    // was created, not a reload of the finished one.
    expect(p1.url()).not.toBe(p1GameUrl)

    // Both players should be in an active game state, not a game-over state.
    const p1Status = await p1.getByTestId('game-status').textContent({ timeout: 10_000 })
    const p2Status = await p2.getByTestId('game-status').textContent({ timeout: 10_000 })
    expect(['Your turn', "Opponent's turn"]).toContain(p1Status?.trim())
    expect(['Your turn', "Opponent's turn"]).toContain(p2Status?.trim())

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('back to lobby button after game ends closes socket and redirects', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    // Wait for game-over state before interacting with navigation.
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })

    // Clicking Back to Lobby calls leaveRoom() and navigates to /.
    // A subsequent room creation should work cleanly, proving the socket
    // was properly closed and a new one can be established.
    await p1.getByRole('button', { name: 'Back to Lobby' }).click()
    await expect(p1).toHaveURL('/', { timeout: 10_000 })
    await expect(p1.getByTestId('create-room-btn')).toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('occupied cell is disabled and cannot be clicked', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)

    // P1 plays cell 0.
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await p1.locator('[data-cell="0"]').click()

    // Cell 0 should now be filled and disabled for both players —
    // the UI prevents replaying an occupied cell at the component level.
    await expect(p1.locator('[data-cell="0"]')).toBeDisabled({ timeout: 10_000 })
    await expect(p2.locator('[data-cell="0"]')).toBeDisabled({ timeout: 10_000 })

    // The turn should have advanced to P2.
    await expect(p2.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})