import { test, expect } from '@playwright/test'
import { createPlayerContexts, setupAndStartGame, playFullGame } from './helpers'

// ---------------------------------------------------------------------------
// Session History & Replay tests
//
// Covers:
//   - "View Replay" button appears after game ends
//   - Navigating to /sessions/:id/history from the game
//   - Stats bar shows correct move count, duration, ended_by
//   - Event log tab shows expected events (game_started, move_applied, game_over)
//   - Event log rows are expandable (payload toggle)
//   - Replay tab renders the board
//   - Replay slider navigates between states
//   - Step 0 shows empty board, step N reflects move N
//   - "Back to Lobby" navigates to /
// ---------------------------------------------------------------------------

test.describe('Session history and replay', () => {
  test('View Replay button appears when game ends', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    // P1 won — View Replay button should appear in the game-over UI.
    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('View Replay navigates to /sessions/:id/history', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()

    await expect(p1).toHaveURL(/\/sessions\/.*\/history/, { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('history page shows stats bar with correct move count', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    // playFullGame plays 5 moves — stats bar should show "5".
    await expect(p1.getByTestId('stat-move-count')).toContainText('5', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('history page shows result badge for winner', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    // P1 won — badge should say WIN.
    await expect(p1.getByTestId('result-badge')).toContainText('WIN', { timeout: 10_000 })

    // P2 lost — badge should say LOSS.
    await p2.getByTestId('view-replay-btn').click()
    await expect(p2).toHaveURL(/\/sessions\/.*\/history/)
    await expect(p2.getByTestId('result-badge')).toContainText('LOSS', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('event log tab shows game_started and game_over events', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    // Event log tab is active by default.
    await expect(p1.getByTestId('tab-events')).toBeVisible()

    // Should contain game_started and game_over entries.
    await expect(p1.locator('[data-testid="event-row"]', { hasText: 'Game started' })).toBeVisible({
      timeout: 10_000,
    })
    await expect(p1.locator('[data-testid="event-row"]', { hasText: 'Game over' })).toBeVisible({
      timeout: 10_000,
    })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('event log shows correct number of move_applied events', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    // playFullGame plays 5 moves — should be 5 "Move played" rows.
    const moveRows = p1.locator('[data-testid="event-row"]', { hasText: 'Move played' })
    await expect(moveRows).toHaveCount(5, { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('event row payload is expandable', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    // Click the toggle on the first move_applied row.
    const firstMoveRow = p1.locator('[data-testid="event-row"]', { hasText: 'Move played' }).first()
    await expect(firstMoveRow).toBeVisible({ timeout: 10_000 })

    const toggle = firstMoveRow.getByTestId('event-toggle')
    await toggle.click()

    // Payload pre should now be visible.
    await expect(firstMoveRow.getByTestId('event-payload')).toBeVisible({ timeout: 5_000 })

    // Click again to collapse.
    await toggle.click()
    await expect(firstMoveRow.getByTestId('event-payload')).not.toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('replay tab renders the board', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    // Switch to Replay tab.
    await p1.getByTestId('tab-replay').click()

    // Board should be visible.
    await expect(p1.locator('[data-cell="0"]')).toBeVisible({ timeout: 10_000 })

    // All cells disabled — replay is read-only.
    for (let i = 0; i < 9; i++) {
      await expect(p1.locator(`[data-cell="${i}"]`)).toBeDisabled({ timeout: 5_000 })
    }

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('replay slider at step 0 shows empty board', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    await p1.getByTestId('tab-replay').click()

    // Step 0 = initial state label.
    await expect(p1.getByTestId('replay-step-label')).toContainText('Initial state', {
      timeout: 10_000,
    })

    // All cells should be empty (no X or O text).
    for (let i = 0; i < 9; i++) {
      await expect(p1.locator(`[data-cell="${i}"]`)).toHaveText('', { timeout: 5_000 })
    }

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('replay next button advances to move 1 and shows cell 0 filled', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    await p1.getByTestId('tab-replay').click()
    await expect(p1.getByTestId('replay-step-label')).toContainText('Initial state', {
      timeout: 10_000,
    })

    // Advance to move 1 — P1 played cell 0.
    await p1.getByTestId('replay-next-btn').click()
    await expect(p1.getByTestId('replay-step-label')).toContainText('Move 1', { timeout: 5_000 })

    // Cell 0 should now show a mark (X or O).
    await expect(p1.locator('[data-cell="0"]')).not.toHaveText('', { timeout: 5_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('replay last button jumps to final state', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    await p1.getByTestId('tab-replay').click()

    // Jump to last move.
    await p1.getByTestId('replay-last-btn').click()
    await expect(p1.getByTestId('replay-step-label')).toContainText('Move 5', { timeout: 5_000 })

    // Cells 0, 1, 2 should be filled (P1 top row win).
    for (const cell of [0, 1, 2]) {
      await expect(p1.locator(`[data-cell="${cell}"]`)).not.toHaveText('', { timeout: 5_000 })
    }

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('Back to Lobby navigates to /', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    await expect(p1.getByTestId('view-replay-btn')).toBeVisible({ timeout: 10_000 })
    await p1.getByTestId('view-replay-btn').click()
    await expect(p1).toHaveURL(/\/sessions\/.*\/history/)

    await p1.getByTestId('back-to-lobby-btn').click()
    await expect(p1).toHaveURL('/', { timeout: 5_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})
