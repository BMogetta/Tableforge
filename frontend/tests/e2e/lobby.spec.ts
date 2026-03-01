import { test, expect } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

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
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))
    await p1.goto('/')
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const code = await p1.getByTestId('room-code').textContent()
    expect(code).toHaveLength(6)

    const p2Ctx = await browser.newContext({ storageState: PLAYER2_STATE })
    const p2 = await p2Ctx.newPage()
    p2.on('console', msg => console.log('P2:', msg.text()))
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
})