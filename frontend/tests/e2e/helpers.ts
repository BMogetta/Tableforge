import { expect, Browser, Page } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const PLAYER1_STATE = path.join(__dirname, '.auth/player1.json')
const PLAYER2_STATE = path.join(__dirname, '.auth/player2.json')
const PLAYER3_STATE = path.join(__dirname, '.auth/player3.json')

export { PLAYER1_STATE, PLAYER2_STATE, PLAYER3_STATE }

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

// Creates a third browser context for player3, used in spectator tests.
export async function createSpectatorContext(browser: Browser) {
  const p3Ctx = await browser.newContext({ storageState: PLAYER3_STATE })
  const p3 = await p3Ctx.newPage()
  p3.on('console', msg => console.log('P3:', msg.text()))
  p3.on('pageerror', err => console.log('P3 ERROR:', err.message))
  await p3.goto('/')
  return { p3Ctx, p3 }
}

// Waits for a player's WebSocket to be fully connected before proceeding.
// Prevents race conditions where P1 starts the game before P2's WS is ready.
export async function waitForSocketConnected(page: Page) {
  await page.waitForFunction(
    () => document.querySelector('[data-socket-status="connected"]') !== null,
  )
}

// Waits for the GameLoading screen to complete and the game board to appear.
// Must be called after navigating to /game/:id — the loading screen sends
// POST /ready and waits for game_ready from the server before showing the board.
export async function waitForGameReady(page: Page) {
  await page.waitForSelector('[data-testid="game-status"]', { timeout: 20_000 })
}

// P1 creates a room, P2 joins via the room code, P1 starts the game.
// Both pages are asserted to have navigated to /game/:id before returning.
//
// first_mover_policy is explicitly set to 'fixed' (seat 0 = P1) before
// starting so that all tests that depend on turn order are deterministic.
export async function setupAndStartGame(p1: Page, p2: Page) {
  const player1Id = process.env.TEST_PLAYER1_ID!

  // Explicitly select TicTacToe before creating the room — the default game
  // is non-deterministic when multiple games are registered.
  await p1.getByTestId('game-option-tictactoe').click()
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const code = await p1.getByTestId('room-code').textContent()
  const roomId = p1.url().split('/rooms/')[1]

  // Force first_mover_policy to 'fixed' so P1 (seat 0) always goes first.
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
    data: { player_id: player1Id, value: 'fixed' },
  })

  await p2.getByTestId('join-code-input').fill(code!)
  await p2.getByTestId('join-btn').click()
  await expect(p2).toHaveURL(/\/rooms\//)

  await waitForSocketConnected(p2)

  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
  await p1.getByTestId('start-game-btn').click()

  await expect(p1).toHaveURL(/\/game\//)
  await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })
  
  // Wait for both players to complete the ready handshake before returning.
  await Promise.all([waitForGameReady(p1), waitForGameReady(p2)])
}

// Enables spectators for a room via the settings API.
// Must be called before starting the game.
export async function enableSpectators(p1: Page, roomId: string) {
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/allow_spectators`, {
    data: { player_id: process.env.TEST_PLAYER1_ID!, value: 'yes' },
  })
}

// Sets room visibility to private via the settings API.
export async function setRoomPrivate(p1: Page, roomId: string) {
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/room_visibility`, {
    data: { player_id: process.env.TEST_PLAYER1_ID!, value: 'private' },
  })
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

  await Promise.all([waitForGameReady(p1), waitForGameReady(p2)])

  for (const { player, cell } of moves) {
    await expect(player.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await player.locator(`[data-cell="${cell}"]`).click()
  }
}
