import { test, expect } from '@playwright/test'
import { getPair, createPlayerContexts, setupAndStartGame, playFullGame } from './helpers'

// ---------------------------------------------------------------------------
// Session History & Replay tests
//
// The game-over transition test needs to observe the game-over screen,
// so it plays its own game.
//
// All other tests share a single game played in beforeAll and navigate
// directly to the history URL — avoiding ~15s of setup per test.
// ---------------------------------------------------------------------------

test.describe('Game-over transition', () => {
  test('View Replay button appears and navigates to /sessions/:id/history', async ({
    browser,
  }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupAndStartGame(p1, p2, pair.p1Id)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()

    await expect(p1).toHaveURL(/\/sessions\/.*\/history/, { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})

test.describe('Session history page', () => {
  let historyUrl: string
  let pair: ReturnType<typeof getPair>

  test.beforeAll(async ({ browser }, testInfo) => {
    pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupAndStartGame(p1, p2, pair.p1Id)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/, { timeout: 10_000 })

    historyUrl = p1.url()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('stats bar shows correct move count', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: pair.p1State })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await expect(page.getByTestId('stat-move-count')).toContainText('5', { timeout: 10_000 })

    await ctx.close()
  })

  test('result badge shows WIN for winner and LOSS for loser', async ({ browser }) => {
    const p1Ctx = await browser.newContext({ storageState: pair.p1State })
    const p1 = await p1Ctx.newPage()
    await p1.goto(historyUrl)

    await expect(p1.getByTestId('result-badge')).toContainText('WIN', { timeout: 10_000 })

    const p2Ctx = await browser.newContext({ storageState: pair.p2State })
    const p2 = await p2Ctx.newPage()
    await p2.goto(historyUrl)
    await expect(p2.getByTestId('result-badge')).toContainText('LOSS', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('event log shows game_started and game_over events', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: pair.p1State })
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
    const ctx = await browser.newContext({ storageState: pair.p1State })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    const moveRows = page.locator('[data-testid="event-row"]', { hasText: 'Move played' })
    await expect(moveRows).toHaveCount(5, { timeout: 10_000 })

    await ctx.close()
  })

  test('event row payload is expandable', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: pair.p1State })
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
    const ctx = await browser.newContext({ storageState: pair.p1State })
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
    const ctx = await browser.newContext({ storageState: pair.p1State })
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
    const ctx = await browser.newContext({ storageState: pair.p1State })
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
    const ctx = await browser.newContext({ storageState: pair.p1State })
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
