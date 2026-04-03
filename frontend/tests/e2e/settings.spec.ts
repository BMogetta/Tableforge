import { test, expect } from '@playwright/test'
import {
  createPlayerContexts,
  setupRoom,
  setupAndStartGame,
  playFullGame,
  waitForSocketConnected,
} from './helpers'

// ---------------------------------------------------------------------------
// Settings E2E tests
//
// Covers:
//   - WS setting_updated propagation to other clients
//   - Read-only view for non-owners
//   - Turn order enforcement via first_mover_policy + first_mover_seat
//   - rematch_first_mover_policy: winner_first, loser_first, fixed
// ---------------------------------------------------------------------------

test.describe('LobbySettings UI', () => {
  test('setting_updated WS event updates the read-only value shown to p2', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupRoom(p1, p2)

    // Wait for both sides to settle before changing the setting.
    await expect(p1.getByTestId('setting-select-first_mover_policy')).toBeVisible({
      timeout: 10_000,
    })
    await expect(p2.getByTestId('setting-value-first_mover_policy')).toBeVisible({
      timeout: 10_000,
    })

    // Owner changes the policy.
    await p1.getByTestId('setting-select-first_mover_policy').selectOption('fixed')

    // P2's read-only span must reflect the new value without a page reload.
    // The label rendered for 'fixed' depends on the option list returned by the
    // backend — assert on the underlying test-id value attribute via the span text.
    await expect(p2.getByTestId('setting-value-first_mover_policy')).not.toContainText('Random', {
      timeout: 10_000,
    })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('non-owner sees settings as read-only (no select element)', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await setupRoom(p1, p2)

    // P2 must see the readonly span, not an interactive select.
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
  test('first_mover_policy "fixed" with seat 0: p1 goes first', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.goto('/')
    await p2.goto('/')

    // Explicitly select TicTacToe — default game is non-deterministic with
    // multiple games registered.
    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    const player1Id = process.env.TEST_PLAYER1_ID!

    // Set fixed policy with seat 0 (p1 goes first).
    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: player1Id, value: 'fixed' },
    })
    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_seat`, {
      data: { player_id: player1Id, value: '0' },
    })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await waitForSocketConnected(p2)

    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()

    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // P1 (seat 0) must be prompted first; P2 must be waiting.
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('first_mover_policy "fixed" with seat 1: p2 goes first', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.goto('/')
    await p2.goto('/')

    // Explicitly select TicTacToe — default game is non-deterministic with
    // multiple games registered.
    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    const player1Id = process.env.TEST_PLAYER1_ID!

    // Set fixed policy with seat 1 (p2 goes first).
    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: player1Id, value: 'fixed' },
    })
    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_seat`, {
      data: { player_id: player1Id, value: '1' },
    })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await waitForSocketConnected(p2)

    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()

    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // P2 (seat 1) must be prompted first; P1 must be waiting.
    await expect(p2.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p1.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})

// ---------------------------------------------------------------------------

// setupRematchLobby sets up a room with the given rematch policy, plays a full
// first game (P1 wins via fixed/seat 0), both players vote rematch, owner
// starts the second game. Both pages are left on /game/ ready for assertions.
async function setupRematchLobby(p1: any, p2: any, rematchPolicy: string): Promise<void> {
  await p1.goto('/')
  await p2.goto('/')

  // Explicitly select TicTacToe — default game is non-deterministic with
  // multiple games registered.
  await p1.getByTestId('game-option-tictactoe').click()
  await p1.getByTestId('create-room-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//)

  const roomId = p1.url().split('/rooms/')[1]
  const code = await p1.getByTestId('room-code').textContent()
  const player1Id = process.env.TEST_PLAYER1_ID!

  // First game: fixed/seat 0 so P1 always goes first and wins the full game.
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
    data: { player_id: player1Id, value: 'fixed' },
  })
  await p1.request.put(`/api/v1/rooms/${roomId}/settings/rematch_first_mover_policy`, {
    data: { player_id: player1Id, value: rematchPolicy },
  })

  await p2.getByTestId('join-code-input').fill(code!)
  await p2.getByTestId('join-btn').click()
  await expect(p2).toHaveURL(/\/rooms\//)

  await waitForSocketConnected(p2)

  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
  await p1.getByTestId('start-game-btn').click()

  await expect(p1).toHaveURL(/\/game\//)
  await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

  // Play first game to completion — P1 wins.
  await playFullGame(p1, p2)
  await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })
  await expect(p2.getByTestId('game-status')).toContainText('You lost', { timeout: 10_000 })

  // Both vote rematch — navigate back to lobby.
  await p1.getByTestId('rematch-btn').click()
  await p2.getByTestId('rematch-btn').click()
  await expect(p1).toHaveURL(/\/rooms\//, { timeout: 15_000 })
  await expect(p2).toHaveURL(/\/rooms\//, { timeout: 10_000 })

  await waitForSocketConnected(p2)

  // Owner starts the rematch.
  await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
  await p1.getByTestId('start-game-btn').click()

  await expect(p1).toHaveURL(/\/game\//)
  await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })
}

test.describe('Rematch first mover policy', () => {
  test('winner_first: the winner of the previous game goes first in the rematch', async ({
    browser,
  }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    // P1 wins the first game. With winner_first, P1 must go first in the rematch.
    await setupRematchLobby(p1, p2, 'winner_first')

    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('loser_first: the loser of the previous game goes first in the rematch', async ({
    browser,
  }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    // P1 wins the first game. With loser_first, P2 must go first in the rematch.
    await setupRematchLobby(p1, p2, 'loser_first')

    await expect(p2.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p1.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('fixed: seat 0 goes first in the rematch regardless of previous result', async ({
    browser,
  }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    // P1 wins the first game. With fixed (default seat 0), P1 goes first again.
    await setupRematchLobby(p1, p2, 'fixed')

    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})
