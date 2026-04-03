import { test, expect } from '@playwright/test'
import {
  getPair,
  createPlayerContexts,
  setupRoom,
  setupAndStartGame,
  playFullGame,
  waitForSocketConnected,
} from './helpers'

test.describe('LobbySettings UI', () => {
  test('setting_updated WS event updates the read-only value shown to p2', async ({ browser }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupRoom(p1, p2)

    await expect(p1.getByTestId('setting-select-first_mover_policy')).toBeVisible({
      timeout: 10_000,
    })
    await expect(p2.getByTestId('setting-value-first_mover_policy')).toBeVisible({
      timeout: 10_000,
    })

    await p1.getByTestId('setting-select-first_mover_policy').selectOption('fixed')

    await expect(p2.getByTestId('setting-value-first_mover_policy')).not.toContainText('Random', {
      timeout: 10_000,
    })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('non-owner sees settings as read-only (no select element)', async ({ browser }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupRoom(p1, p2)

    await expect(p2.getByTestId('setting-value-first_mover_policy')).toBeVisible({
      timeout: 10_000,
    })
    await expect(p2.getByTestId('setting-select-first_mover_policy')).not.toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })
})

// ---------------------------------------------------------------------------

test.describe('Turn order enforcement', () => {
  test('first_mover_policy "fixed" with seat 0: p1 goes first', async ({ browser }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await p1.goto('/')
    await p2.goto('/')

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: pair.p1Id, value: 'fixed' },
    })
    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_seat`, {
      data: { player_id: pair.p1Id, value: '0' },
    })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await waitForSocketConnected(p2)

    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()

    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('first_mover_policy "fixed" with seat 1: p2 goes first', async ({ browser }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await p1.goto('/')
    await p2.goto('/')

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: pair.p1Id, value: 'fixed' },
    })
    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_seat`, {
      data: { player_id: pair.p1Id, value: '1' },
    })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await waitForSocketConnected(p2)

    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()

    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    await expect(p2.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p1.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})

// ---------------------------------------------------------------------------

async function setupRematchLobby(p1: any, p2: any, p1Id: string, rematchPolicy: string): Promise<void> {
  await p1.goto('/')
  await p2.goto('/')

  await p1.getByTestId('game-option-tictactoe').click()
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const roomId = p1.url().split('/rooms/')[1]
  const code = await p1.getByTestId('room-code').textContent()

  await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
    data: { player_id: p1Id, value: 'fixed' },
  })
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/rematch_first_mover_policy`, {
    data: { player_id: p1Id, value: rematchPolicy },
  })

  await p2.getByTestId('join-code-input').fill(code!)
  await p2.getByTestId('join-btn').click()
  await expect(p2).toHaveURL(/\/rooms\//)

  await waitForSocketConnected(p2)

  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
  await p1.getByTestId('start-game-btn').click()

  await expect(p1).toHaveURL(/\/game\//)
  await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

  await playFullGame(p1, p2)
  await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })
  await expect(p2.getByTestId('game-status')).toContainText('You lost', { timeout: 10_000 })

  await p1.getByTestId('rematch-btn').click()
  await p2.getByTestId('rematch-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//, { timeout: 15_000 })
  await expect(p2).toHaveURL(/\/rooms\//, { timeout: 10_000 })

  await waitForSocketConnected(p2)

  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
  await p1.getByTestId('start-game-btn').click()

  await expect(p1).toHaveURL(/\/game\//)
  await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })
}

test.describe('Rematch first mover policy', () => {
  test('winner_first: the winner of the previous game goes first in the rematch', async ({
    browser,
  }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupRematchLobby(p1, p2, pair.p1Id, 'winner_first')

    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('loser_first: the loser of the previous game goes first in the rematch', async ({
    browser,
  }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupRematchLobby(p1, p2, pair.p1Id, 'loser_first')

    await expect(p2.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p1.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('fixed: seat 0 goes first in the rematch regardless of previous result', async ({
    browser,
  }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupRematchLobby(p1, p2, pair.p1Id, 'fixed')

    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})
