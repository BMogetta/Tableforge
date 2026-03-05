import { expect, Browser, Page } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const PLAYER1_STATE = path.join(__dirname, '.auth/player1.json')
const PLAYER2_STATE = path.join(__dirname, '.auth/player2.json')

export { PLAYER1_STATE, PLAYER2_STATE }

// --- Shared helpers ----------------------------------------------------------

// Creates two authenticated browser contexts and navigates both to the lobby.
export async function createPlayerContexts(browser: Browser) {
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
//
// first_mover_policy is explicitly set to 'fixed' (seat 0 = P1) before
// starting so that all tests that depend on turn order are deterministic.
export async function setupAndStartGame(p1: Page, p2: Page) {
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const code = await p1.getByTestId('room-code').textContent()

  // Extract the room ID from the URL so we can call the settings API directly.
  const roomUrl = p1.url()
  const roomId = roomUrl.split('/rooms/')[1]

  // Read player1 ID from the env var injected by the test runner.
  const player1Id = process.env.TEST_PLAYER1_ID!

  // Force first_mover_policy to 'fixed' so P1 (seat 0) always goes first.
  // This makes all turn-order assertions in tests deterministic regardless
  // of the room default (which is 'random').
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
    data: { player_id: player1Id, value: 'fixed' },
  })

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
export async function playFullGame(p1: Page, p2: Page) {
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