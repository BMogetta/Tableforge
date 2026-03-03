import { test, expect } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'
import { createPlayerContexts } from './helpers'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const PLAYER1_STATE = path.join(__dirname, '.auth/player1.json')
const PLAYER2_STATE = path.join(__dirname, '.auth/player2.json')

test.describe('Lobby', () => {
  test('shows lobby after login', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()

    await page.goto('/')
    await expect(page.getByTestId('create-room-btn')).toBeVisible()
    await expect(page.getByTestId('join-code-input')).toBeVisible()
    await expect(page.getByTestId('join-btn')).toBeVisible()
    await ctx.close()
  })

  test('can create a room', async ({ browser }) => {
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()

    await page.goto('/')
    await page.getByTestId('create-room-btn').click()
    await expect(page).toHaveURL(/\/rooms\//)
    await expect(page.getByTestId('room-code')).toBeVisible()
    await ctx.close()
  })

  test('two players can join the same room', async ({ browser }) => {
    const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const p1 = await p1Ctx.newPage()
    await p1.goto('/')
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const code = await p1.getByTestId('room-code').textContent()
    expect(code).toHaveLength(6)

    const p2Ctx = await browser.newContext({ storageState: PLAYER2_STATE })
    const p2 = await p2Ctx.newPage()
    await p2.goto('/')
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    // start-game-btn exists in DOM but is disabled until P2 joins.
    // When P2 joins via WS event, Room refreshes and canStart becomes true.
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('player leaving room disables start button for host', async ({ browser }) => {
    const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const p1 = await p1Ctx.newPage()

    const p2Ctx = await browser.newContext({ storageState: PLAYER2_STATE })
    const p2 = await p2Ctx.newPage()

    await p1.goto('/')
    await p2.goto('/')

    // P1 creates a room, P2 joins — start button becomes enabled.
    await p1.getByTestId('create-room-btn').click()
    const code = await p1.getByTestId('room-code').textContent()
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    // P2 leaves the room. The player_left WS event should cause P1's room view
    // to refresh, dropping the player count and disabling the start button again.
    await p2.getByRole('button', { name: 'Leave' }).click()
    await expect(p2).toHaveURL('/', { timeout: 10_000 })

    await expect(p1.getByTestId('start-game-btn')).toBeDisabled({ timeout: 10_000 })
    await expect(p1.getByTestId('player-count')).toContainText('1/', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('room disappears from lobby after game ends', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('create-room-btn').click()
    const code = await p1.getByTestId('room-code').textContent()

    // Wait for P2's lobby to poll and show the new room.
    await expect(async () => {
      await p2.reload()
      await expect(p2.locator(`text=${code}`)).toBeVisible()
    }).toPass({ timeout: 15_000 })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()
    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // Play full game — P1 wins.
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

    // Navigate back to lobby and verify the room is gone.
    await p1.goto('/')
    await expect(p1.locator(`text=${code}`)).not.toBeVisible({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('owner leaving transfers host to remaining player', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('create-room-btn').click()
    const code = await p1.getByTestId('room-code').textContent()
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    // P1 (owner) leaves — P2 should become host and see the start button.
    await p1.getByRole('button', { name: 'Leave' }).click()
    await expect(p1).toHaveURL('/', { timeout: 10_000 })

    await expect(p2.getByTestId('start-game-btn')).toBeVisible({ timeout: 10_000 })
    await expect(p2.locator('span', { hasText: 'Host' })).toBeVisible({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('last player leaving closes the room', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('create-room-btn').click()
    const code = await p1.getByTestId('room-code').textContent()
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    // P2 leaves first.
    await p2.getByRole('button', { name: 'Leave' }).click()
    await expect(p2).toHaveURL('/', { timeout: 10_000 })

    // P1 (last player) leaves — room should be destroyed.
    await p1.getByRole('button', { name: 'Leave' }).click()
    await expect(p1).toHaveURL('/', { timeout: 10_000 })

    // Lobby polls every 10s — wait for the next refresh cycle.
    await p1.waitForTimeout(11_000)
    await expect(p1.locator(`text=${code}`)).not.toBeVisible()
    await expect(p2.locator(`text=${code}`)).not.toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('non-owner leaving does not change host', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('create-room-btn').click()
    const code = await p1.getByTestId('room-code').textContent()
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    // P2 (non-owner) leaves — P1 should still be host with start button enabled.
    await p2.getByRole('button', { name: 'Leave' }).click()
    await expect(p2).toHaveURL('/', { timeout: 10_000 })

    await expect(p1.getByTestId('start-game-btn')).toBeDisabled({ timeout: 10_000 })
    await expect(p1.locator('span', { hasText: 'Host' })).toBeVisible({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})