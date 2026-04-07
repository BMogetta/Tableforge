import { test, expect } from './fixtures'
import {
  setupRoom,
  waitForSocketConnected,
  playFullGame,
} from './helpers'

test.describe('Lobby', () => {
  test('two players can join the same room', async ({ players }) => {
    const { p1, p2 } = players

    const { code } = await setupRoom(p1, p2)
    expect(code).toHaveLength(6)

    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
  })

  test('player leaving room disables start button for host', async ({ players }) => {
    const { p1, p2 } = players

    await setupRoom(p1, p2)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    await p2.getByRole('button', { name: 'Leave' }).click()
    await expect(p2).toHaveURL('/', { timeout: 10_000 })

    await expect(p1.getByTestId('start-game-btn')).toBeDisabled({ timeout: 10_000 })
    await expect(p1.getByTestId('player-count')).toContainText('1/', { timeout: 10_000 })
  })

  test('room disappears from lobby after game ends', async ({ players }) => {
    const { p1, p2, p1Id } = players

    await p1.getByTestId('game-option-tictactoe').click()
    await p1.getByTestId('create-room-btn').click()
    const code = await p1.getByTestId('room-code').textContent()

    const roomId = p1.url().split('/rooms/')[1]
    await p1.request.put(`/api/v1/rooms/${roomId}/settings/first_mover_policy`, {
      data: { player_id: p1Id, value: 'fixed' },
    })

    await expect(async () => {
      await p2.reload()
      await expect(p2.locator(`text=${code}`)).toBeVisible()
    }).toPass({ timeout: 15_000 })

    await p2.getByTestId('join-code-input').fill(code!)
    await p2.getByTestId('join-btn').click()

    await waitForSocketConnected(p2)

    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })
    await p1.getByTestId('start-game-btn').click()
    await expect(p1).toHaveURL(/\/game\//)
    await expect(p2).toHaveURL(/\/game\//, { timeout: 10_000 })

    await playFullGame(p1, p2)
    await expect(p1.getByTestId('game-status')).toContainText('You won', { timeout: 10_000 })

    await p1.goto('/')
    await expect(p1.locator(`text=${code}`)).not.toBeVisible({ timeout: 10_000 })
  })

  test('owner leaving transfers host to remaining player', async ({ players }) => {
    const { p1, p2 } = players

    await setupRoom(p1, p2)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    await p1.getByRole('button', { name: 'Leave' }).click()
    await expect(p1).toHaveURL('/', { timeout: 10_000 })

    await expect(p2.getByTestId('start-game-btn')).toBeVisible({ timeout: 10_000 })
    await expect(p2.locator('span', { hasText: 'Host' })).toBeVisible({ timeout: 10_000 })
  })

  test('last player leaving closes the room', async ({ players }) => {
    const { p1, p2 } = players

    const { code } = await setupRoom(p1, p2)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    await p2.getByRole('button', { name: 'Leave' }).click()
    await expect(p2).toHaveURL('/', { timeout: 10_000 })

    await p1.getByRole('button', { name: 'Leave' }).click()
    await expect(p1).toHaveURL('/', { timeout: 10_000 })

    await expect(async () => {
      await p1.reload()
      await expect(p1.locator(`text=${code}`)).not.toBeVisible()
    }).toPass({ timeout: 15_000 })
    await expect(p2.locator(`text=${code}`)).not.toBeVisible()
  })

  test('non-owner leaving does not change host', async ({ players }) => {
    const { p1, p2 } = players

    await setupRoom(p1, p2)
    await expect(p1.getByTestId('start-game-btn')).toBeEnabled({ timeout: 10_000 })

    await p2.getByRole('button', { name: 'Leave' }).click()
    await expect(p2).toHaveURL('/', { timeout: 10_000 })

    await expect(p1.getByTestId('start-game-btn')).toBeDisabled({ timeout: 10_000 })
    await expect(p1.locator('span', { hasText: 'Host' })).toBeVisible({ timeout: 10_000 })
  })
})
