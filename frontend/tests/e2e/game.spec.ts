import { test, expect } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const PLAYER1_STATE = path.join(__dirname, '.auth/player1.json')
const PLAYER2_STATE = path.join(__dirname, '.auth/player2.json')

test.describe('TicTacToe game', () => {
  test('two players can play a full game', async ({ browser }) => {
    const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))
    await p1.goto('/')
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const code = await p1.getByTestId('room-code').textContent()

    const p2Ctx = await browser.newContext({ storageState: PLAYER2_STATE })
    const p2 = await p2Ctx.newPage()
    await p2.goto('/')
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()

    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // P1 wins top row: 0, 1, 2 — P2 plays 3, 4
    const moves = [
      { player: p1, cell: 0 },
      { player: p2, cell: 3 },
      { player: p1, cell: 1 },
      { player: p2, cell: 4 },
      { player: p1, cell: 2 },
    ]

    for (const { player, cell } of moves) {
      await expect(player.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
      await player.locator(`[data-cell="${cell}"]`).click()
    }

    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You lost', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('turn timeout ends the game', async ({ browser }) => {
    const p1Ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))
    await p1.goto('/')
    await p1.getByTestId('create-room-btn').click()
    const code = await p1.getByTestId('room-code').textContent()

    const p2Ctx = await browser.newContext({ storageState: PLAYER2_STATE })
    const p2 = await p2Ctx.newPage()
    await p2.goto('/')
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()

    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // P1 does nothing — wait for timeout (requires default_timeout_secs=10 in game_configs)
    await expect(p1.getByTestId('game-status')).toContainText('You lost', { timeout: 35_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You won', { timeout: 35_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})