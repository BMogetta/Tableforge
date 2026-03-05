import { test, expect } from '@playwright/test'
import { createPlayerContexts, playFullGame, setupAndStartGame } from './helpers'

// --- Tests -------------------------------------------------------------------

test.describe('TicTacToe game', () => {
  test('two players can play a full game', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    // Assert final game-over state is correctly shown to both players.
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You lost', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('turn timeout ends the game', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)

    // Neither player moves. The server's turn timer fires after default_timeout_secs (30s)
    // and broadcasts game_over with the idle player as the loser.
    // Timeout is 35s to give the server a few seconds of margin.
    await expect(p1.getByTestId('game-status')).toContainText('You lost', { timeout: 35_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You won', { timeout: 35_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('player can forfeit a game', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)

    // Clicking ← Lobby mid-game should open a confirmation modal instead of
    // navigating immediately, to prevent accidental forfeits.
    await p1.locator('button', { hasText: '← Lobby' }).click()
    await expect(p1.getByRole('dialog')).toBeVisible()
    await expect(p1.getByRole('dialog')).toContainText('Forfeit game?')

    // Cancelling dismisses the modal and keeps the player in the game.
    await p1.getByRole('button', { name: 'Cancel' }).click()
    await expect(p1.getByRole('dialog')).not.toBeVisible()
    await expect(p1).toHaveURL(/\/game\//)

    // Confirming the forfeit ends the game: P1 is redirected to the lobby
    // and P2 receives game_over via WS with a win result.
    await p1.locator('button', { hasText: '← Lobby' }).click()
    await expect(p1.getByRole('dialog')).toBeVisible()
    await p1.getByTestId('confirm-surrender-btn').click()

    await expect(p1).toHaveURL('/', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('both players can rematch after a game ends', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    // Wait for game-over state to settle on both sides before interacting
    // with the rematch button — the WS game_over event must have been processed.
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You lost', { timeout: 10_000 })

    const p1GameUrl = p1.url()

    // P1 votes for rematch first. With only one vote in, the button should
    // switch to a waiting indicator and P2's button should remain active.
    await p1.getByTestId('rematch-btn').click()
    await expect(p1.getByTestId('rematch-btn')).toBeDisabled()
    await expect(p1.getByTestId('rematch-btn')).toContainText('Waiting')

    // P2 receives the rematch_vote WS event — their button stays enabled.
    await expect(p2.getByTestId('rematch-btn')).toBeEnabled()

    // P2 votes. Both players have now voted — the server resets the room to
    // waiting and broadcasts rematch_ready. Both clients navigate to /rooms/:id.
    await p2.getByTestId('rematch-btn').click()

    await expect(p1).not.toHaveURL(p1GameUrl, { timeout: 15_000 })
    await expect(p1).toHaveURL(/\/rooms\//, { timeout: 10_000 })
    await expect(p2).toHaveURL(/\/rooms\//, { timeout: 10_000 })

    // Both players should be in the same room (same URL).
    expect(p1.url()).toBe(p2.url())

    // The room should be back in waiting state — owner sees the start button
    // (disabled until both players are present, which they already are).
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('rematch full flow: vote, return to lobby, start second game', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    // Use fixed policy so p1 always goes first — deterministic assertions.
    await p1.goto('/')
    await p2.goto('/')
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    const player1Id = process.env.TEST_PLAYER1_ID!

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: player1Id, value: 'fixed' },
    })
    await p1.request.put(`/api/v1/rooms/${roomId}/settings/rematch_first_mover_policy`, {
      data: { player_id: player1Id, value: 'fixed' },
    })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()

    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // Play the first game — P1 wins.
    await playFullGame(p1, p2)
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).toContainText('You lost', { timeout: 10_000 })

    // Both vote for rematch — return to lobby.
    await p1.getByTestId('rematch-btn').click()
    await p2.getByTestId('rematch-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//, { timeout: 15_000 })
    await expect(p2).toHaveURL(/\/rooms\//, { timeout: 10_000 })

    // Owner starts the second game from the lobby.
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()

    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // rematch_first_mover_policy is 'fixed' (seat 0) — p1 goes first again.
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await expect(p2.getByTestId('game-status')).not.toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('back to lobby button after game ends closes socket and redirects', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)
    await playFullGame(p1, p2)

    // Wait for game-over state before interacting with navigation.
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })

    // Clicking Back to Lobby calls leaveRoom() and navigates to /.
    // A subsequent room creation should work cleanly, proving the socket
    // was properly closed and a new one can be established.
    await p1.getByRole('button', { name: 'Back to Lobby' }).click()
    await expect(p1).toHaveURL('/', { timeout: 10_000 })
    await expect(p1.getByTestId('create-room-btn')).toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('occupied cell is disabled and cannot be clicked', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    await setupAndStartGame(p1, p2)

    // P1 plays cell 0.
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await p1.locator('[data-cell="0"]').click()

    // Cell 0 should now be filled and disabled for both players —
    // the UI prevents replaying an occupied cell at the component level.
    await expect(p1.locator('[data-cell="0"]')).toBeDisabled({ timeout: 10_000 })
    await expect(p2.locator('[data-cell="0"]')).toBeDisabled({ timeout: 10_000 })

    // The turn should have advanced to P2.
    await expect(p2.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})