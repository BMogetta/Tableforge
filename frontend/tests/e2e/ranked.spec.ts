import { test, expect } from './fixtures'
import { waitForGameReady } from './helpers'
import type { Page } from '@playwright/test'

/**
 * Plays a full TicTacToe game without assuming who moves first.
 * The "first" player (whoever has "Your turn") wins with the top row.
 */
async function playRankedGame(p1: Page, p2: Page) {
  await Promise.all([waitForGameReady(p1), waitForGameReady(p2)])

  const p1First = await p1
    .getByTestId('game-status')
    .textContent({ timeout: 10_000 })
    .then(t => t?.includes('Your turn') ?? false)

  const first = p1First ? p1 : p2
  const second = p1First ? p2 : p1

  const moves = [
    { player: first, cell: 0 },
    { player: second, cell: 3 },
    { player: first, cell: 1 },
    { player: second, cell: 4 },
    { player: first, cell: 2 },
  ]

  for (const { player, cell } of moves) {
    await expect(player.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await player.locator(`[data-cell="${cell}"]`).click()
  }
}

/** Fetch display_rating for a player via API. */
async function getRating(page: Page, playerId: string, gameId = 'tictactoe') {
  const res = await page.request.get(`/api/v1/players/${playerId}/ratings/${gameId}`)
  if (!res.ok()) return null
  const data = await res.json()
  return data.display_rating as number
}

/** Fetch games_played for a player (0 if no rating row exists yet). */
async function getGamesPlayed(page: Page, playerId: string, gameId = 'tictactoe') {
  const res = await page.request.get(`/api/v1/players/${playerId}/ratings/${gameId}`)
  if (!res.ok()) return 0
  const data = await res.json()
  return (data.games_played as number) ?? 0
}

/** Ensure both players are on the lobby page. Handles game-over screens from cleanup forfeits. */
async function ensureLobby(...pages: Page[]) {
  for (const page of pages) {
    // If stuck on a game-over screen (forfeit from cleanup), dismiss it first.
    const backBtn = page.getByTestId('back-to-lobby-btn')
    if (await backBtn.isVisible().catch(() => false)) {
      await backBtn.click()
      await expect(page).toHaveURL('/', { timeout: 10_000 })
    }

    const onLobby = await page.getByTestId('game-option-tictactoe').isVisible().catch(() => false)
    if (!onLobby) {
      await page.goto('/')
      await expect(page.getByTestId('game-option-tictactoe')).toBeVisible({ timeout: 10_000 })
    }
  }
}

/**
 * Full ranked cycle: both players queue from the lobby, accept the match,
 * play a game to completion, wait for rating update, then return to lobby.
 */
async function queueAndPlayRankedGame(p1: Page, p2: Page, p1Id: string, p2Id: string) {
  // Ensure both players are on lobby before starting.
  await ensureLobby(p1, p2)

  // Reset queue state from any previous run (test-mode only endpoint).
  await Promise.all([
    p1.request.delete(`/api/v1/queue/players/${p1Id}/state`),
    p1.request.delete(`/api/v1/queue/players/${p2Id}/state`),
  ])

  // Capture games_played BEFORE the game — avoids race with rating-service.
  const gamesBefore = await getGamesPlayed(p1, p1Id)

  // Select game and ranked tab.
  await p1.getByTestId('game-option-tictactoe').click()
  await p2.getByTestId('game-option-tictactoe').click()
  await p1.getByTestId('tab-ranked').click()
  await p2.getByTestId('tab-ranked').click()

  // Join queue — wait for both enqueue POSTs to complete so the ticker
  // sees both players before firing. Otherwise the first tick can race
  // between the two clicks.
  const isEnqueue = (url: string, method: string) =>
    method === 'POST' && /\/api\/v1\/queue(\?|$)/.test(url)
  await Promise.all([
    p1.waitForResponse(r => isEnqueue(r.url(), r.request().method()) && r.ok()),
    p2.waitForResponse(r => isEnqueue(r.url(), r.request().method()) && r.ok()),
    p1.getByTestId('find-match-btn').click(),
    p2.getByTestId('find-match-btn').click(),
  ])

  // Wait for match found on both pages in parallel.
  await Promise.all([
    expect(p1.getByTestId('match-found')).toBeVisible({ timeout: 30_000 }),
    expect(p2.getByTestId('match-found')).toBeVisible({ timeout: 30_000 }),
  ])

  // Both accept — parallel to avoid match expiry between clicks.
  await Promise.all([
    p1.getByTestId('accept-match-btn').click(),
    p2.getByTestId('accept-match-btn').click(),
  ])

  // Navigate to game.
  await expect(p1).toHaveURL(/\/game\//, { timeout: 15_000 })
  await expect(p2).toHaveURL(/\/game\//, { timeout: 15_000 })

  // Play to completion.
  await playRankedGame(p1, p2)

  // Verify game over.
  await expect(p1.getByTestId('game-status')).toContainText(/won|lost|Draw/, { timeout: 10_000 })
  await expect(p2.getByTestId('game-status')).toContainText(/won|lost|Draw/, { timeout: 10_000 })

  // Ranked game-over UI contract: no rematch button (would bypass MMR seeding),
  // "back to queue" is offered instead so players re-match with fresh pairing.
  await expect(p1.getByTestId('rematch-btn')).toHaveCount(0)
  await expect(p2.getByTestId('rematch-btn')).toHaveCount(0)
  await expect(p1.getByTestId('back-to-queue-btn')).toBeVisible()
  await expect(p2.getByTestId('back-to-queue-btn')).toBeVisible()

  // Wait for rating-service to process (async via Redis Pub/Sub).
  await expect.poll(
    async () => getGamesPlayed(p1, p1Id),
    { timeout: 15_000, message: 'games_played should increase after rated game' },
  ).toBeGreaterThan(gamesBefore)

  // Return to lobby.
  await p1.getByTestId('back-to-lobby-btn').click()
  await p2.getByTestId('back-to-lobby-btn').click()
  await expect(p1).toHaveURL('/', { timeout: 10_000 })
  await expect(p2).toHaveURL('/', { timeout: 10_000 })
}

// --- Tests -------------------------------------------------------------------

test.describe('Ranked matchmaking', () => {
  test.describe.configure({ mode: 'serial' })
  test('two players queue, match, play, and ratings change', async ({ rankedPlayers }) => {
    const { p1, p2, p1Id, p2Id } = rankedPlayers

    const [ratingP1Before, ratingP2Before] = await Promise.all([
      getRating(p1, p1Id),
      getRating(p2, p2Id),
    ])

    await queueAndPlayRankedGame(p1, p2, p1Id, p2Id)

    await expect.poll(
      async () => getRating(p1, p1Id),
      { timeout: 15_000, message: 'P1 rating should change' },
    ).not.toBe(ratingP1Before)

    await expect.poll(
      async () => getRating(p2, p2Id),
      { timeout: 15_000, message: 'P2 rating should change' },
    ).not.toBe(ratingP2Before)
  })

  test('five ranked games unlock leaderboard', async ({ rankedPlayers }) => {
    test.setTimeout(180_000)

    const { p1, p2, p1Id, p2Id } = rankedPlayers

    for (let i = 0; i < 5; i++) {
      await queueAndPlayRankedGame(p1, p2, p1Id, p2Id)
    }

    // Both players should now have games_played >= 5 (leaderboardMinGames).
    await p1.getByTestId('game-option-tictactoe').click()

    await expect(p1.getByTestId('leaderboard-table')).toBeVisible({ timeout: 15_000 })

    // Verify both players appear and have divergent ratings.
    const [ratingP1, ratingP2] = await Promise.all([
      getRating(p1, p1Id),
      getRating(p1, p2Id),
    ])

    expect(ratingP1).not.toBeNull()
    expect(ratingP2).not.toBeNull()
    // Ratings should differ — one won more than the other.
    expect(ratingP1).not.toBe(ratingP2)
  })

  test('player can decline a match', async ({ rankedPlayers }) => {
    const { p1, p2, p1Id, p2Id } = rankedPlayers

    // Ensure players are on the lobby (cleanup may leave them on game-over).
    await ensureLobby(p1, p2)

    // Reset queue state from any previous run.
    await Promise.all([
      p1.request.delete(`/api/v1/queue/players/${p1Id}/state`),
      p1.request.delete(`/api/v1/queue/players/${p2Id}/state`),
    ])

    await p1.getByTestId('game-option-tictactoe').click()
    await p2.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('tab-ranked').click()
    await p2.getByTestId('tab-ranked').click()

    await p1.getByTestId('find-match-btn').click()
    await p2.getByTestId('find-match-btn').click()

    await expect(p1.getByTestId('match-found')).toBeVisible({ timeout: 30_000 })
    await expect(p2.getByTestId('match-found')).toBeVisible({ timeout: 30_000 })

    await p1.getByTestId('accept-match-btn').click()
    await p2.getByTestId('decline-match-btn').click()

    // P2 returns to idle.
    await expect(p2.getByTestId('find-match-btn')).toBeVisible({ timeout: 10_000 })

    // P1 should be re-queued (searching) or back to idle if re-queue didn't happen.
    // Accept both outcomes — the key assertion is that P1 is NOT stuck on match-found.
    await expect(
      p1.getByTestId('queue-searching').or(p1.getByTestId('find-match-btn')),
    ).toBeVisible({ timeout: 15_000 })
  })
})
