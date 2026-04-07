import { test, expect } from './fixtures'
import { setupRoom, setupAndStartGame } from './helpers'

test.describe('Player presence', () => {
  test('presence dot shown in room player list', async ({ players }) => {
    const { p1, p2, p2Id } = players

    await setupRoom(p1, p2)

    await expect(p1.locator(`[data-testid="presence-dot-${p2Id}"]`)).toHaveAttribute(
      'data-online',
      'true',
      { timeout: 10_000 },
    )
  })

  test('presence dot goes offline when player leaves room', async ({ players }) => {
    const { p1, p2, p2Id } = players

    await setupRoom(p1, p2)

    await expect(p1.locator(`[data-testid="presence-dot-${p2Id}"]`)).toHaveAttribute(
      'data-online',
      'true',
      { timeout: 10_000 },
    )

    // P2 navigates away — WS closes, presence deleted.
    await p2.goto('/')

    await expect(p1.locator(`[data-testid="presence-dot-${p2Id}"]`)).toHaveAttribute(
      'data-online',
      'false',
      { timeout: 30_000 },
    )
  })

  test('opponent presence indicator shown during game', async ({ players }) => {
    const { p1, p2, p1Id } = players

    await setupAndStartGame(p1, p2, p1Id)

    await expect(p1.getByTestId('opponent-presence-dot')).toHaveAttribute('data-online', 'true', {
      timeout: 10_000,
    })
    await expect(p2.getByTestId('opponent-presence-dot')).toHaveAttribute('data-online', 'true', {
      timeout: 10_000,
    })
  })

  test('opponent presence goes offline when opponent disconnects during game', async ({
    players,
  }) => {
    const { p1, p2, p1Id, p2Ctx } = players

    await setupAndStartGame(p1, p2, p1Id)

    await expect(p1.getByTestId('opponent-presence-dot')).toHaveAttribute('data-online', 'true', {
      timeout: 10_000,
    })

    // P2 closes their context — simulates disconnect.
    await p2Ctx.close()

    await expect(p1.getByTestId('opponent-presence-dot')).toHaveAttribute('data-online', 'false', {
      timeout: 20_000,
    })
  })

  test('opponent presence text updates correctly', async ({ players }) => {
    const { p1, p2, p1Id, p2Ctx } = players

    await setupAndStartGame(p1, p2, p1Id)

    await expect(p1.getByTestId('opponent-presence-text')).toContainText('Opponent online', {
      timeout: 10_000,
    })

    // P2 disconnects.
    await p2Ctx.close()

    await expect(p1.getByTestId('opponent-presence-text')).toContainText('Opponent offline', {
      timeout: 30_000,
    })
  })
})
