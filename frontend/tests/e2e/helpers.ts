import { expect, Browser, Page } from '@playwright/test'
import { fileURLToPath } from 'url'
import fs from 'fs'
import path from 'path'

const __dirname = path.dirname(fileURLToPath(import.meta.url))

// --- Player pool -------------------------------------------------------------
//
// Each Playwright project gets its own pair of players so tests can run in
// parallel (workers: 4) without sharing game state.
//
//   Pair 1 (P1–P2):   game-tests, session-history-tests, chromium-parallel
//   Pair 2 (P3–P4):   chat-tests
//   Pair 3 (P5–P6):   settings-tests
//   Pair 4 (P7–P8):   spectator-tests  (P9 = spectator)
//   Pair 5 (P10–P11): presence-tests

const playersJson: Record<string, string> = JSON.parse(
  fs.readFileSync(path.join(__dirname, '.players.json'), 'utf-8'),
)

function playerState(n: number) {
  return path.join(__dirname, `.auth/player${n}.json`)
}

function playerId(n: number) {
  return playersJson[`player${n}_id`]
}

// Player pair assignments keyed by Playwright project name.
const PROJECT_PAIRS: Record<string, { p1: number; p2: number; spectator?: number }> = {
  'game-tests': { p1: 1, p2: 2 },
  'session-history-tests': { p1: 1, p2: 2 },
  'chromium-parallel': { p1: 1, p2: 2 },
  'chat-tests': { p1: 3, p2: 4 },
  'settings-tests': { p1: 5, p2: 6 },
  'spectator-tests': { p1: 7, p2: 8, spectator: 9 },
  'presence-tests': { p1: 10, p2: 11 },
}

// Backwards-compat exports for files that import these directly.
export const PLAYER1_STATE = playerState(1)
export const PLAYER2_STATE = playerState(2)
export const PLAYER3_STATE = playerState(3)

export interface PlayerPair {
  p1State: string
  p2State: string
  p1Id: string
  p2Id: string
  spectatorState?: string
  spectatorId?: string
}

/** Resolve the player pair for a given Playwright project name. */
export function getPair(projectName: string): PlayerPair {
  const pair = PROJECT_PAIRS[projectName]
  if (!pair) throw new Error(`No player pair configured for project "${projectName}"`)
  return {
    p1State: playerState(pair.p1),
    p2State: playerState(pair.p2),
    p1Id: playerId(pair.p1),
    p2Id: playerId(pair.p2),
    spectatorState: pair.spectator ? playerState(pair.spectator) : undefined,
    spectatorId: pair.spectator ? playerId(pair.spectator) : undefined,
  }
}

// --- Shared helpers ----------------------------------------------------------

/** Creates two authenticated browser contexts and navigates both to the lobby. */
export async function createPlayerContexts(browser: Browser, pair: PlayerPair) {
  const p1Ctx = await browser.newContext({ storageState: pair.p1State })
  const p1 = await p1Ctx.newPage()
  p1.on('console', msg => console.log('P1:', msg.text()))
  p1.on('pageerror', err => console.log('P1 ERROR:', err.message))

  const p2Ctx = await browser.newContext({ storageState: pair.p2State })
  const p2 = await p2Ctx.newPage()
  p2.on('console', msg => console.log('P2:', msg.text()))
  p2.on('pageerror', err => console.log('P2 ERROR:', err.message))

  await p1.goto('/')
  await p2.goto('/')

  return { p1Ctx, p1, p2Ctx, p2 }
}

/** Creates a third browser context for the spectator player. */
export async function createSpectatorContext(browser: Browser, pair: PlayerPair) {
  if (!pair.spectatorState) throw new Error('No spectator configured for this pair')
  const p3Ctx = await browser.newContext({ storageState: pair.spectatorState })
  const p3 = await p3Ctx.newPage()
  p3.on('console', msg => console.log('P3:', msg.text()))
  p3.on('pageerror', err => console.log('P3 ERROR:', err.message))
  await p3.goto('/')
  return { p3Ctx, p3 }
}

/** Waits for a player's WebSocket to be fully connected before proceeding. */
export async function waitForSocketConnected(page: Page) {
  await page.waitForFunction(
    () => document.querySelector('[data-socket-status="connected"]') !== null,
  )
}

/** Waits for the GameLoading screen to complete and the game board to appear. */
export async function waitForGameReady(page: Page) {
  await page.waitForSelector('[data-testid="game-status"]', { timeout: 20_000 })
  await page.waitForFunction(
    () => document.querySelector('[data-socket-status="connected"]') !== null,
    { timeout: 20_000 },
  )
}

/** P1 creates a TicTacToe room, P2 joins via the room code. Does NOT start. */
export async function setupRoom(p1: Page, p2: Page) {
  await p1.getByTestId('game-option-tictactoe').click()
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const code = await p1.getByTestId('room-code').textContent()
  const roomId = p1.url().split('/rooms/')[1]

  await p2.getByTestId('join-code-input').fill(code!)
  await p2.getByTestId('join-btn').click()
  await expect(p2).toHaveURL(/\/rooms\//)

  return { roomId, code: code! }
}

/**
 * P1 creates a room, P2 joins, P1 starts the game.
 * first_mover_policy is set to 'fixed' (seat 0 = P1) for deterministic turns.
 */
export async function setupAndStartGame(p1: Page, p2: Page, p1Id: string) {
  await p1.getByTestId('game-option-tictactoe').click()
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const code = await p1.getByTestId('room-code').textContent()
  const roomId = p1.url().split('/rooms/')[1]

  await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
    data: { player_id: p1Id, value: 'fixed' },
  })

  await p2.getByTestId('join-code-input').fill(code!)
  await p2.getByTestId('join-btn').click()
  await expect(p2).toHaveURL(/\/rooms\//)

  await waitForSocketConnected(p2)

  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
  await p1.getByTestId('start-game-btn').click()

  await expect(p1).toHaveURL(/\/game\//)
  await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

  await Promise.all([waitForGameReady(p1), waitForGameReady(p2)])
}

/** Enables spectators for a room via the settings API. */
export async function enableSpectators(p1: Page, roomId: string, p1Id: string) {
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/allow_spectators`, {
    data: { player_id: p1Id, value: 'yes' },
  })
}

/** Sets room visibility to private via the settings API. */
export async function setRoomPrivate(p1: Page, roomId: string, p1Id: string) {
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/room_visibility`, {
    data: { player_id: p1Id, value: 'private' },
  })
}

/**
 * Plays a fixed winning sequence for TicTacToe.
 * P1 wins the top row (cells 0, 1, 2). P2 plays cells 3 and 4.
 */
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
