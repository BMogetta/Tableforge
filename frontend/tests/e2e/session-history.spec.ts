import { test, expect } from '@playwright/test'
import {
  PLAYER1_STATE,
  PLAYER2_STATE,
  createPlayerContexts,
  setupAndStartGame,
  playFullGame,
} from './helpers'

// ---------------------------------------------------------------------------
// Session History & Replay tests
//
// The game-over transition tests (View Replay button, navigation) need to
// observe the game-over screen, so they play their own game.
//
// All other tests share a single game played in beforeAll and navigate
// directly to the history URL — avoiding ~15s of setup per test.
// ---------------------------------------------------------------------------

test.describe('Game-over transition', () => {
  test('View Replay button appears and navigates to /sessions/:id/history', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    // P1 won — View Replay button should appear in the game-over UI.
    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()

    await expect(p1).toHaveURL(/\/sessions\/.*\/history/, { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})

test.describe('Session history page', () => {
  let historyUrl: string

  test.beforeAll(async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/, { timeout: 10_000 })

    historyUrl = p1.url()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('stats bar shows correct move count', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    // playFullGame plays 5 moves — stats bar should show "5".
    await expect(page.getByTestId('stat-move-count')).toContainText('5', { timeout: 10_000 })

    await ctx.close()
  })

  test('result badge shows WIN for winner and LOSS for loser', async ({ browser }) => {
    const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const p1 = await p1Ctx.newPage()
    await p1.goto(historyUrl)

    // P1 won — badge should say WIN.
    await expect(p1.getByTestId('result-badge')).toContainText('WIN', { timeout: 10_000 })

    // P2 lost — badge should say LOSS.
    const p2Ctx = await browser.newContext({ storageState: PLAYER2_STATE })
    const p2 = await p2Ctx.newPage()
    await p2.goto(historyUrl)
    await expect(p2.getByTestId('result-badge')).toContainText('LOSS', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('event log shows game_started and game_over events', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    // Event log tab is active by default.
    await expect(page.getByTestId('tab-events')).toBeVisible()

    // Should contain game_started and game_over entries.
    await expect(
      page.locator('[data-testid="event-row"]', { hasText: 'Game started' }),
    ).toBeVisible({ timeout: 10_000 })
    await expect(page.locator('[data-testid="event-row"]', { hasText: 'Game over' })).toBeVisible({
      timeout: 10_000,
    })

    await ctx.close()
  })

  test('event log shows correct number of move_applied events', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    // playFullGame plays 5 moves — should be 5 "Move played" rows.
    const moveRows = page.locator('[data-testid="event-row"]', { hasText: 'Move played' })
    await expect(moveRows).toHaveCount(5, { timeout: 10_000 })

    await ctx.close()
  })

  test('event row payload is expandable', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    // Click the toggle on the first move_applied row.
    const firstMoveRow = page
      .locator('[data-testid="event-row"]', { hasText: 'Move played' })
      .first()
    await expect(firstMoveRow).toBeVisible({ timeout: 10_000 })

    const toggle = firstMoveRow.getByTestId('event-toggle')
    await toggle.click()

    // Payload pre should now be visible.
    await expect(firstMoveRow.getByTestId('event-payload')).toBeVisible({ timeout: 5_000 })

    // Click again to collapse.
    await toggle.click()
    await expect(firstMoveRow.getByTestId('event-payload')).not.toBeVisible()

    await ctx.close()
  })

  test('replay tab renders the board with all cells disabled', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    // Switch to Replay tab.
    await page.getByTestId('tab-replay').click()

    // Board should be visible.
    await expect(page.locator('[data-cell="0"]')).toBeVisible({ timeout: 10_000 })

    // All cells disabled — replay is read-only.
    for (let i = 0; i < 9; i++) {
      await expect(page.locator(`[data-cell="${i}"]`)).toBeDisabled({ timeout: 5_000 })
    }

    await ctx.close()
  })

  test('replay slider at step 0 shows empty board', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await page.getByTestId('tab-replay').click()

    // Step 0 = initial state label.
    await expect(page.getByTestId('replay-step-label')).toContainText('Initial state', {
      timeout: 10_000,
    })

    // All cells should be empty (no X or O text).
    for (let i = 0; i < 9; i++) {
      await expect(page.locator(`[data-cell="${i}"]`)).toHaveText('', { timeout: 5_000 })
    }

    await ctx.close()
  })

  test('replay next button advances to move 1 and shows cell 0 filled', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await page.getByTestId('tab-replay').click()
    await expect(page.getByTestId('replay-step-label')).toContainText('Initial state', {
      timeout: 10_000,
    })

    // Advance to move 1 — P1 played cell 0.
    await page.getByTestId('replay-next-btn').click()
    await expect(page.getByTestId('replay-step-label')).toContainText('Move 1', { timeout: 5_000 })

    // Cell 0 should now show a mark (X or O).
    await expect(page.locator('[data-cell="0"]')).not.toHaveText('', { timeout: 5_000 })

    await ctx.close()
  })

  test('replay last button jumps to final state', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()
    await page.goto(historyUrl)

    await page.getByTestId('tab-replay').click()

    // Jump to last move.
    await page.getByTestId('replay-last-btn').click()
    await expect(page.getByTestId('replay-step-label')).toContainText('Move 5', { timeout: 5_000 })

    // Cells 0, 1, 2 should be filled (P1 top row win).
    for (const cell of [0, 1, 2]) {
      await expect(page.locator(`[data-cell="${cell}"]`)).not.toHaveText('', { timeout: 5_000 })
    }

    await ctx.close()
  })
})
