import { test, expect } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const PLAYER1_STATE = path.join(__dirname, '.auth/player1.json')

// The game ID that was used when seeding ratings in seed-test.
const GAME_ID = 'tictactoe'

test('leaderboard shows seeded players with display_rating', async ({ browser }) => {
  const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
  const p1 = await p1Ctx.newPage()

  await p1.goto('/')

  // Read P1's username from the header.
  await expect(p1.getByTestId('player-username')).toBeVisible({ timeout: 10_000 })
  const p1Username = await p1.getByTestId('player-username').textContent()

  // The leaderboard fetches for effectiveGame which defaults to the first
  // game in the registry (tictactoe). If the game selector is visible,
  // click the tictactoe option to be explicit.
  const gameOption = p1.locator(`button`, { hasText: /tictactoe/i })
  if ((await gameOption.count()) > 0) {
    await gameOption.first().click()
  }

  // Wait for the leaderboard table to appear.
  await expect(p1.getByTestId('leaderboard-table')).toBeVisible({ timeout: 15_000 })

  // Verify P1 appears in the leaderboard — seeded with display_rating=1536.
  const rows = p1.getByTestId('leaderboard-row')
  await expect(rows.first()).toBeVisible({ timeout: 10_000 })

  const count = await rows.count()
  let found = false
  for (let i = 0; i < count; i++) {
    const nameCell = await rows.nth(i).locator('td').nth(1).textContent()
    if (nameCell?.trim() === p1Username?.trim()) {
      found = true
      // Verify display_rating column is a number (not wins/losses/draws).
      const ratingCell = await rows.nth(i).locator('td').nth(2).textContent()
      const rating = parseInt(ratingCell?.trim() ?? '0', 10)
      expect(rating).toBeGreaterThan(0)
      break
    }
  }
  expect(found).toBe(true)

  await p1Ctx.close()
})
