import { test, expect, Browser } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const PLAYER1_STATE = path.join(__dirname, '.auth/player1.json')
const PLAYER2_STATE = path.join(__dirname, '.auth/player2.json')

test('reflects wins and losses after a completed game', async ({ browser }) => {
  const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
  const p1 = await p1Ctx.newPage()
  const p2Ctx = await browser.newContext({ storageState: PLAYER2_STATE })
  const p2 = await p2Ctx.newPage()

  await p1.goto('/')
  await p2.goto('/')

  // Read P1's username from the header.
  await expect(p1.getByTestId('player-username')).toBeVisible({ timeout: 10_000 })
  const p1Username = await p1.getByTestId('player-username').textContent()

  // Baseline wins — if the table doesn't exist yet, player has 0 wins.
  const getWins = async (username: string) => {
    const tableExists = await p1.getByTestId('leaderboard-table').count()
    if (!tableExists) return 0
    const rows = p1.getByTestId('leaderboard-row')
    const count = await rows.count()
    for (let i = 0; i < count; i++) {
      const row = rows.nth(i)
      const nameCell = await row.locator('td').nth(1).textContent()
      if (nameCell?.trim() === username?.trim()) {
        const wins = await row.locator('td').nth(2).textContent()
        return wins ? parseInt(wins.trim(), 10) : 0
      }
    }
    return 0
  }

  const winsBefore = await getWins(p1Username!)

  // Create room and force first_mover_policy to 'fixed' (seat 0 = P1) before
  // starting so the move sequence below is always valid regardless of the
  // room default (which is 'random').
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)
  const code = await p1.getByTestId('room-code').textContent()

  const roomId = p1.url().split('/rooms/')[1]
  const res = await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
    data: { player_id: process.env.TEST_PLAYER1_ID!, value: 'fixed' },
  })
  expect(res.status()).toBe(204)

  await p2.getByTestId('join-code-input').fill(code!)
  await p2.getByTestId('join-btn').click()
  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
  await p1.getByTestId('start-game-btn').click()
  await expect(p1).toHaveURL(/\/game\//)
  await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

  // Play full game — P1 wins top row.
  const moves = [
    { player: p1, cell: 0 }, { player: p2, cell: 3 },
    { player: p1, cell: 1 }, { player: p2, cell: 4 },
    { player: p1, cell: 2 },
  ]
  for (const { player, cell } of moves) {
    await expect(player.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await player.locator(`[data-cell="${cell}"]`).click()
  }
  await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })

  // Navigate back to lobby — fresh mount triggers leaderboard refetch.
  await p1.goto('/')

  // Wait for the leaderboard table to appear (it may not exist before the first game).
  await expect(p1.getByTestId('leaderboard-table')).toBeVisible({ timeout: 15_000 })

  // Poll until P1's win count reflects the completed game.
  await expect(async () => {
    const winsAfter = await getWins(p1Username!)
    expect(winsAfter).toBeGreaterThan(winsBefore)
  }).toPass({ timeout: 15_000 })

  await p1Ctx.close()
  await p2Ctx.close()
})