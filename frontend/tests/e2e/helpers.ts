import { expect, type Page } from '@playwright/test'

// --- Shared helpers ----------------------------------------------------------

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

  await waitForSocketConnected(p2)

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
