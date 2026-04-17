import { expect, test } from './fixtures'
import { enableSpectators, playFullGame, setRoomPrivate, waitForSocketConnected } from './helpers'

test.describe('Spectator mode', () => {
  test('spectator is rejected when allow_spectators is "no"', async ({ playersWithSpectator }) => {
    const { p1, p3, p1Id, p3Id } = playersWithSpectator

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]

    // allow_spectators defaults to 'no' — the WS upgrade should be rejected.
    const res = await p3.request.get(`/ws/rooms/${roomId}?player_id=${p3Id}`)
    expect(res.status()).toBe(403)
  })

  test('spectator can join a room with allow_spectators "yes"', async ({
    playersWithSpectator,
  }) => {
    const { p1, p3, p1Id } = playersWithSpectator

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    await enableSpectators(p1, roomId, p1Id)

    await p3.goto(`/rooms/${roomId}`)
    await expect(p3).toHaveURL(/\/rooms\//)
    await expect(p3.locator('span', { hasText: 'Spectating' })).toBeVisible({ timeout: 10_000 })
  })

  test('spectator count updates when a spectator joins and leaves', async ({
    playersWithSpectator,
  }) => {
    const { p1, p2, p3, p1Id } = playersWithSpectator

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await enableSpectators(p1, roomId, p1Id)

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await expect(p1.getByTestId('spectator-count')).not.toBeVisible()

    await p3.goto(`/rooms/${roomId}`)
    await expect(p3.locator('span', { hasText: 'Spectating' })).toBeVisible({ timeout: 15_000 })

    await expect(p1.getByTestId('spectator-count')).toBeVisible({ timeout: 15_000 })
    await expect(p1.getByTestId('spectator-count')).toContainText('1', { timeout: 10_000 })
    await expect(p2.getByTestId('spectator-count')).toContainText('1', { timeout: 10_000 })

    await p3.goto('/')

    await expect(p1.getByTestId('spectator-count')).not.toBeVisible({ timeout: 15_000 })
    await expect(p2.getByTestId('spectator-count')).not.toBeVisible({ timeout: 15_000 })
  })

  test('spectator sees game board but cannot make moves', async ({ playersWithSpectator }) => {
    const { p1, p2, p3, p1Id } = playersWithSpectator

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await enableSpectators(p1, roomId, p1Id)

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: p1Id, value: 'fixed' },
    })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    await p3.goto(`/rooms/${roomId}`)
    await expect(p3.locator('span', { hasText: 'Spectating' })).toBeVisible({ timeout: 15_000 })

    await waitForSocketConnected(p2)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()
    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    // Spectator may not receive the game_started WS redirect reliably.
    // If they don't navigate automatically, follow the game URL directly.
    const gameUrl = p1.url()
    try {
      await expect(p3).toHaveURL(/\/game\//, { timeout: 10_000 })
    } catch {
      await p3.goto(gameUrl)
    }

    await expect(p3.locator('[data-cell="0"]')).toBeVisible({ timeout: 10_000 })
    for (let i = 0; i < 9; i++) {
      await expect(p3.locator(`[data-cell="${i}"]`)).toBeDisabled({ timeout: 5000 })
    }

    await expect(p3.getByTestId('game-status')).not.toContainText('Your turn')
  })

  test('spectator sees live move updates via WS', async ({ playersWithSpectator }) => {
    const { p1, p2, p3, p1Id } = playersWithSpectator

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await enableSpectators(p1, roomId, p1Id)

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: p1Id, value: 'fixed' },
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

    // Spectator may not receive the game_started WS redirect reliably.
    const gameUrl = p1.url()
    try {
      await expect(p3).toHaveURL(/\/game\//, { timeout: 10_000 })
    } catch {
      await p3.goto(gameUrl)
    }

    await expect(p1.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await p1.locator('[data-cell="0"]').click()
    await expect(p3.locator('[data-cell="0"]')).toBeDisabled({ timeout: 10_000 })

    await expect(p2.getByTestId('game-status')).toContainText('Your turn', { timeout: 10_000 })
    await p2.locator('[data-cell="3"]').click()
    await expect(p3.locator('[data-cell="3"]')).toBeDisabled({ timeout: 10_000 })
  })

  test('spectator does not see a rematch button after the game ends', async ({
    playersWithSpectator,
  }) => {
    const { p1, p2, p3, p1Id } = playersWithSpectator

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await enableSpectators(p1, roomId, p1Id)

    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: p1Id, value: 'fixed' },
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

    // Spectator may not receive the game_started WS redirect reliably.
    const gameUrl = p1.url()
    try {
      await expect(p3).toHaveURL(/\/game\//, { timeout: 10_000 })
    } catch {
      await p3.goto(gameUrl)
    }

    await playFullGame(p1, p2)
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })

    await expect(p1.getByTestId('rematch-btn')).toBeVisible({ timeout: 10_000 })
    await expect(p3.getByTestId('rematch-btn')).not.toBeVisible({ timeout: 10_000 })
  })
})

// ---------------------------------------------------------------------------

test.describe('Private rooms', () => {
  test('private room code is hidden in the lobby list', async ({ players }) => {
    const { p1, p2, p1Id } = players

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()

    await setRoomPrivate(p1, roomId, p1Id)

    await expect(async () => {
      await p2.reload()
      await expect(p2.getByTestId('lobby-room-list')).toBeVisible()
      await expect(p2.locator('[data-testid="room-card"]').first()).toBeVisible()
    }).toPass({ timeout: 15_000 })

    await expect(p2.locator(`text=${code}`)).not.toBeVisible()

    await expect(p2.locator('[data-testid="room-card-private-icon"]').first()).toBeVisible()
  })

  test('private room has no direct join button in the lobby', async ({ players }) => {
    const { p1, p2, p1Id } = players

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    await setRoomPrivate(p1, roomId, p1Id)

    await expect(async () => {
      await p2.reload()
      await expect(p2.locator('[data-testid="room-card-private-icon"]').first()).toBeVisible()
    }).toPass({ timeout: 15_000 })

    const privateCard = p2
      .locator('[data-testid="room-card"]')
      .filter({
        has: p2.locator('[data-testid="room-card-private-icon"]'),
      })
      .first()
    await expect(privateCard.getByRole('button', { name: 'Join →' })).not.toBeVisible()
  })

  test('private room can be joined by entering the code manually', async ({ players }) => {
    const { p1, p2, p1Id } = players

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await setRoomPrivate(p1, roomId, p1Id)

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()
    await expect(p2).toHaveURL(/\/rooms\//)

    expect(p1.url()).toBe(p2.url())
  })

  test('private room owner sees the code inside the room view', async ({ players }) => {
    const { p1, p1Id } = players

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()
    await setRoomPrivate(p1, roomId, p1Id)

    // Open invite code popover to see the code.
    await p1.getByTestId('toolbar-invite').click()
    await expect(p1.getByTestId('room-code-display')).toBeVisible({ timeout: 10_000 })
    await expect(p1.getByTestId('room-code-display')).toContainText(code!, { timeout: 5000 })
  })

  test('public room shows code and join button in lobby', async ({ players }) => {
    const { p1, p2 } = players

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const code = await p1.getByTestId('room-code').textContent()

    await expect(async () => {
      await p2.reload()
      await expect(p2.locator('[data-testid="room-card-code"]', { hasText: code! })).toBeVisible()
    }).toPass({ timeout: 15_000 })

    const publicCard = p2.locator('[data-testid="room-card"]').filter({
      has: p2.locator('[data-testid="room-card-code"]', { hasText: code! }),
    })
    await expect(publicCard.getByRole('button', { name: 'Join →' })).toBeVisible()
  })

  test('private room setting can be changed back to public', async ({ players }) => {
    const { p1, p2, p1Id } = players

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    await expect(p1).toHaveURL(/\/rooms\//)

    const roomId = p1.url().split('/rooms/')[1]
    const code = await p1.getByTestId('room-code').textContent()

    await setRoomPrivate(p1, roomId, p1Id)

    // Open settings popover to access room_visibility select.
    await p1.getByTestId('toolbar-settings').click()

    await expect(p1.getByTestId('setting-select-room_visibility')).toHaveValue('private', {
      timeout: 10_000,
    })

    await p1.getByTestId('setting-select-room_visibility').selectOption('public')

    await expect(async () => {
      await p2.reload()
      await expect(p2.locator('[data-testid="room-card-code"]', { hasText: code! })).toBeVisible()
    }).toPass({ timeout: 15_000 })
  })
})
