import { test, expect } from '@playwright/test'
import {
  createPlayerContexts,
  createSpectatorContext,
  enableSpectators,
  setRoomPrivate,
  playFullGame,
  waitForSocketConnected,
} from './helpers'

// ---------------------------------------------------------------------------
// Spectator mode tests
//
// Covers:
//   - Spectator can join a room with allow_spectators=yes
//   - Spectator is rejected (403) when allow_spectators=no
//   - Spectator count badge appears and updates in real time
//   - Spectator sees game board but cannot make moves
//   - Spectator sees live move updates via WS
//   - Spectator does not see a rematch button after the game ends
//
// Private room tests:
//   - Private room code is hidden in the lobby list
//   - Private room has no "Join →" button in the lobby
//   - Private room can be joined by entering the code manually
//   - Private room owner sees the code inside the room view
//   - Public room shows code and join button
//   - Private room setting can be changed back to public
// ---------------------------------------------------------------------------

test.describe('Spectator mode', () => {
  test('spectator is rejected when allow_spectators is "no"', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    const { p3Ctx, p3 } = await createSpectatorContext(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]

    // allow_spectators defaults to 'no' — the WS upgrade should be rejected.
    const res = await p3.request.get(`/ws/rooms/${roomId}?player_id=${process.env.TEST_PLAYER3_ID}`)
    expect(res.status()).toBe(403)

    await p1Ctx.close()
    await p2Ctx.close()
    await p3Ctx.close()
  })

  test('spectator can join a room with allow_spectators "yes"', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    const { p3Ctx, p3 } = await createSpectatorContext(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    await enableSpectators(p1, roomId)

    // P3 navigates directly to the room URL — not in room_players → spectator.
    await p3.goto(`/rooms/${roomId}`)
    await expect(p3).toHaveURL(/\/rooms\//)
    await expect(p3.locator('span', { hasText: 'Spectating' })).toBeVisible({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
    await p3Ctx.close()
  })

  test('spectator count updates when a spectator joins and leaves', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    const { p3Ctx, p3 } = await createSpectatorContext(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await enableSpectators(p1, roomId)

    // P2 joins as participant.
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    // Initially no spectators — badge not visible.
    await expect(p1.getByTestId('spectator-count')).not.toBeVisible()

    // P3 joins as spectator.
    await p3.goto(`/rooms/${roomId}`)
    await expect(p3.locator('span', { hasText: 'Spectating' })).toBeVisible({ timeout: 10_000 })

    // P1 and P2 should see "1 watching".
    await expect(p1.getByTestId('spectator-count')).toBeVisible({ timeout: 10_000 })
    await expect(p1.getByTestId('spectator-count')).toContainText('1', { timeout: 10_000 })
    await expect(p2.getByTestId('spectator-count')).toContainText('1', { timeout: 10_000 })

    // P3 navigates away — WS disconnects, spectator_left is broadcast.
    await p3.goto('/')

    // Badge should disappear.
    await expect(p1.getByTestId('spectator-count')).not.toBeVisible({ timeout: 10_000 })
    await expect(p2.getByTestId('spectator-count')).not.toBeVisible({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
    await p3Ctx.close()
  })

  test('spectator sees game board but cannot make moves', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    const { p3Ctx, p3 } = await createSpectatorContext(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await enableSpectators(p1, roomId)

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: process.env.TEST_PLAYER1_ID!, value: 'fixed' },
    })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await p3.goto(`/rooms/${roomId}`)
    await expect(p3.locator('span', { hasText: 'Spectating' })).toBeVisible({ timeout: 10_000 })

    await waitForSocketConnected(p2)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()
    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // Spectator navigates to /game/ via game_started WS event.
    await expect(p3).toHaveURL(/\/game\//, { timeout: 10_000 })

    // Board is visible but all cells are disabled for the spectator.
    await expect(p3.locator('[data-cell="0"]')).toBeVisible({ timeout: 10_000 })
    for (let i = 0; i < 9; i++) {
      await expect(p3.locator(`[data-cell="${i}"]`)).toBeDisabled({ timeout: 5_000 })
    }

    // Spectator does not see "Your turn".
    await expect(p3.getByTestId('game-status')).not.toContainText('Your turn')

    await p1Ctx.close()
    await p2Ctx.close()
    await p3Ctx.close()
  })

  test('spectator sees live move updates via WS', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    const { p3Ctx, p3 } = await createSpectatorContext(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await enableSpectators(p1, roomId)

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: process.env.TEST_PLAYER1_ID!, value: 'fixed' },
    })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await p3.goto(`/rooms/${roomId}`)
    await expect(p3.locator('span', { hasText: 'Spectating' })).toBeVisible({ timeout: 10_000 })

    await waitForSocketConnected(p2)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()
    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })
    await expect(p3).toHaveURL(/\/game\//, { timeout: 10_000 })

    // P1 plays cell 0 — spectator should see it become disabled.
    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await p1.locator('[data-cell="0"]').click()
    await expect(p3.locator('[data-cell="0"]')).toBeDisabled({ timeout: 10_000 })

    // P2 plays cell 3.
    await expect(p2.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await p2.locator('[data-cell="3"]').click()
    await expect(p3.locator('[data-cell="3"]')).toBeDisabled({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
    await p3Ctx.close()
  })

  test('spectator does not see a rematch button after the game ends', async ({ browser }) => {
    // The rematch button is a participant-only UI element — Game.tsx renders it
    // only when isSpectator is false.
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)
    const { p3Ctx, p3 } = await createSpectatorContext(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await enableSpectators(p1, roomId)

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: process.env.TEST_PLAYER1_ID!, value: 'fixed' },
    })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await p3.goto(`/rooms/${roomId}`)
    await expect(p3.locator('span', { hasText: 'Spectating' })).toBeVisible({ timeout: 10_000 })

    await waitForSocketConnected(p2)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()
    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })
    await expect(p3).toHaveURL(/\/game\//, { timeout: 10_000 })

    await playFullGame(p1, p2)
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })

    // Participants see the rematch button — spectator must not.
    await expect(p1.getByTestId('rematch-btn')).toBeVisible({ timeout: 10_000 })
    await expect(p3.getByTestId('rematch-btn')).not.toBeVisible({ timeout: 10_000 })

    await p1Ctx.close()
    await p2Ctx.close()
    await p3Ctx.close()
  })
})

// ---------------------------------------------------------------------------

test.describe('Private rooms', () => {
  test('private room code is hidden in the lobby list', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()

    await setRoomPrivate(p1, roomId)

    // Wait for P2's lobby to show the room list with at least one entry.
    await expect(async () => {
      await p2.reload()
      await expect(p2.getByTestId('lobby-room-list')).toBeVisible()
      await expect(p2.locator('[data-testid="room-card"]').first()).toBeVisible()
    }).toPass({ timeout: 15_000 })

    // The code must not appear anywhere in P2's lobby.
    await expect(p2.locator(`text=${code}`)).not.toBeVisible()

    // The private icon must be present for at least one card.
    await expect(p2.locator('[data-testid="room-card-private-icon"]').first()).toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('private room has no direct join button in the lobby', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    await setRoomPrivate(p1, roomId)

    // Wait for the private icon to appear in P2's lobby.
    await expect(async () => {
      await p2.reload()
      await expect(p2.locator('[data-testid="room-card-private-icon"]').first()).toBeVisible()
    }).toPass({ timeout: 15_000 })

    // The private room card must not contain a "Join →" button.
    const privateCard = p2
      .locator('[data-testid="room-card"]')
      .filter({
        has: p2.locator('[data-testid="room-card-private-icon"]'),
      })
      .first()
    await expect(privateCard.getByRole('button', { name: 'Join →' })).not.toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('private room can be joined by entering the code manually', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await setRoomPrivate(p1, roomId)

    // P2 enters the code directly — the only way to join a private room.
    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    expect(p1.url()).toBe(p2.url())

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('private room owner sees the code inside the room view', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await setRoomPrivate(p1, roomId)

    // Owner is a participant — the private invite section shows the code.
    await expect(p1.getByTestId('room-code-display')).toBeVisible({ timeout: 10_000 })
    await expect(p1.getByTestId('room-code-display')).toContainText(code!, { timeout: 5_000 })

    // The "Private Room" label must be shown in the invite section.
    await expect(p1.locator('p.label', { hasText: '🔒 Private Room' })).toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('public room shows code and join button in lobby', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    // Visibility defaults to public.
    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const code = await p1.getByTestId('room-code').textContent()

    // Wait for P2's lobby to show the room.
    await expect(async () => {
      await p2.reload()
      await expect(p2.locator('[data-testid="room-card-code"]', { hasText: code! })).toBeVisible()
    }).toPass({ timeout: 15_000 })

    // The public room card must show the code and a Join button.
    const publicCard = p2.locator('[data-testid="room-card"]').filter({
      has: p2.locator('[data-testid="room-card-code"]', { hasText: code! }),
    })
    await expect(publicCard.getByRole('button', { name: 'Join →' })).toBeVisible()

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('private room setting can be changed back to public', async ({ browser }) => {
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser)

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()

    await setRoomPrivate(p1, roomId)

    // Wait for the settings UI to reflect private.
    await expect(p1.getByTestId('setting-select-room_visibility')).toHaveValue('private', {
      timeout: 10_000,
    })

    // Flip back to public.
    await p1.getByTestId('setting-select-room_visibility').selectOption('public')

    // P2's lobby should now show the code.
    await expect(async () => {
      await p2.reload()
      await expect(p2.locator('[data-testid="room-card-code"]', { hasText: code! })).toBeVisible()
    }).toPass({ timeout: 15_000 })

    await p1Ctx.close()
    await p2Ctx.close()
  })
})
