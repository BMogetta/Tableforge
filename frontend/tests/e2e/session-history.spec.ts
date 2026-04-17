import { cleanupPlayer, expect, test } from './fixtures'
import { playFullGame, setupAndStartGame } from './helpers'
import { acquirePlayers, type PoolPlayer, releasePlayers } from './player-pool'

// ---------------------------------------------------------------------------
// Session History & Replay tests
//
// The game-over transition test needs to observe the game-over screen,
// so it plays its own game using the pool fixture.
//
// All other tests share a single game played in beforeAll and navigate
// directly to the history URL — avoiding ~15s of setup per test.
// Serial mode ensures beforeAll runs once (one worker) and pool players
// don't race with the fixture-based tests.
// ---------------------------------------------------------------------------

test.describe('Game-over transition', () => {
  test('View Replay button appears and navigates to /sessions/:id/history', async ({ players }) => {
    const { p1, p2, p1Id } = players

    await setupAndStartGame(p1, p2, p1Id)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()

    await expect(p1).toHaveURL(/\/sessions\/.*\/history/, { timeout: 10_000 })
  })
})

// For the shared session tests, we acquire players manually in beforeAll.
// Serial mode pins all tests to one worker so beforeAll/afterAll run once.
const POOL_TEST_ID = 'session-history-shared'
let pool: PoolPlayer[] = []
let historyUrl: string
let p1StatePath: string

test.describe('Session history page', () => {
  test.describe.configure({ mode: 'serial' })

  test.beforeAll(async ({ browser }) => {
    pool = acquirePlayers(2, POOL_TEST_ID)

    const p1Ctx = await browser.newContext({ storageState: pool[0].statePath })
    const p1 = await p1Ctx.newPage()
    const p2Ctx = await browser.newContext({ storageState: pool[1].statePath })
    const p2 = await p2Ctx.newPage()

    await Promise.all([cleanupPlayer(p1, pool[0].id), cleanupPlayer(p2, pool[1].id)])

    await p1.goto('/')
    await p2.goto('/')

    await setupAndStartGame(p1, p2, pool[0].id)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/, { timeout: 10_000 })

    historyUrl = p1.url()
    p1StatePath = pool[0].statePath

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test.afterAll(() => {
    if (pool.length > 0) releasePlayers(pool, POOL_TEST_ID)
  })

  test('stats bar shows correct move count', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: p1StatePath })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await expect(page.getByTestId('stat-move-count')).toContainText('5', { timeout: 10_000 })

    await ctx.close()
  })

  test('result badge shows WIN for winner and LOSS for loser', async ({ browser }) => {
    const p1Ctx = await browser.newContext({ storageState: pool[0].statePath })
    const p1 = await p1Ctx.newPage()
    await p1.goto(historyUrl)

    await expect(p1.getByTestId('result-badge')).toContainText('WIN', { timeout: 10_000 })

    const p2Ctx = await browser.newContext({ storageState: pool[1].statePath })
    const p2 = await p2Ctx.newPage()
    await p2.goto(historyUrl)
    await expect(p2.getByTestId('result-badge')).toContainText('LOSS', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('event log shows game_started and game_over events', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: p1StatePath })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await expect(page.getByTestId('tab-events')).toBeVisible()

    await expect(
      page.locator('[data-testid="event-row"]', { hasText: 'Game started' }),
    ).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('[data-testid="event-row"]', { hasText: 'Game over' })).toBeVisible({
      timeout: 10_000,
    })

    await ctx.close()
  })

  test('event log shows correct number of move_applied events', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: p1StatePath })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    const moveRows = page.locator('[data-testid="event-row"]', { hasText: 'Move played' })
    await expect(moveRows).toHaveCount(5, { timeout: 10_000 })

    await ctx.close()
  })

  test('event row payload is expandable', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: p1StatePath })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    const firstMoveRow = page
      .locator('[data-testid="event-row"]', { hasText: 'Move played' })
      .first()
    await expect(firstMoveRow).toBeVisible({ timeout: 10_000 })

    const toggle = firstMoveRow.getByTestId('event-toggle')
    await toggle.click()

    await expect(firstMoveRow.getByTestId('event-payload')).toBeVisible({ timeout: 5000 })

    await toggle.click()
    await expect(firstMoveRow.getByTestId('event-payload')).not.toBeVisible()

    await ctx.close()
  })

  test('replay tab renders the board with all cells disabled', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: p1StatePath })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await page.getByTestId('tab-replay').click()

    await expect(page.locator('[data-cell="0"]')).toBeVisible({ timeout: 10_000 })

    for (let i = 0; i < 9; i++) {
      await expect(page.locator(`[data-cell="${i}"]`)).toBeDisabled({ timeout: 5000 })
    }

    await ctx.close()
  })

  test('replay slider at step 0 shows empty board', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: p1StatePath })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await page.getByTestId('tab-replay').click()

    await expect(page.getByTestId('replay-step-label')).toContainText('Initial state', {
      timeout: 10_000,
    })

    for (let i = 0; i < 9; i++) {
      await expect(page.locator(`[data-cell="${i}"]`)).toHaveText('', { timeout: 5000 })
    }

    await ctx.close()
  })

  test('replay next button advances to move 1 and shows cell 0 filled', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: p1StatePath })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await page.getByTestId('tab-replay').click()
    await expect(page.getByTestId('replay-step-label')).toContainText('Initial state', {
      timeout: 10_000,
    })

    await page.getByTestId('replay-next-btn').click()
    await expect(page.getByTestId('replay-step-label')).toContainText('Move 1', { timeout: 5000 })

    await expect(page.locator('[data-cell="0"]')).not.toHaveText('', { timeout: 5000 })

    await ctx.close()
  })

  test('replay last button jumps to final state', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: p1StatePath })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await page.getByTestId('tab-replay').click()

    await page.getByTestId('replay-last-btn').click()
    await expect(page.getByTestId('replay-step-label')).toContainText('Move 5', { timeout: 5000 })

    for (const cell of [0, 1, 2]) {
      await expect(page.locator(`[data-cell="${cell}"]`)).not.toHaveText('', { timeout: 5000 })
    }

    await ctx.close()
  })
})
