import { test, expect } from '@playwright/test'
import { getPair, createPlayerContexts, setupRoom, setupAndStartGame } from './helpers'

test.describe('Player presence', () => {
  test('presence dot shown in room player list', async ({ browser }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupRoom(p1, p2)

    await expect(p1.locator(`[data-testid="presence-dot-${pair.p2Id}"]`)).toHaveAttribute(
      'data-online',
      'true',
      { timeout: 10_000 },
    )

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('presence dot goes offline when player leaves room', async ({ browser }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupRoom(p1, p2)

    await expect(p1.locator(`[data-testid="presence-dot-${pair.p2Id}"]`)).toHaveAttribute(
      'data-online',
      'true',
      { timeout: 10_000 },
    )

    // P2 navigates away — WS closes, presence deleted.
    await p2.goto('/')

    await expect(p1.locator(`[data-testid="presence-dot-${pair.p2Id}"]`)).toHaveAttribute(
      'data-online',
      'false',
      { timeout: 30_000 },
    )

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('opponent presence indicator shown during game', async ({ browser }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupAndStartGame(p1, p2, pair.p1Id)

    await expect(p1.getByTestId('opponent-presence-dot')).toHaveAttribute('data-online', 'true', {
      timeout: 10_000,
    })
    await expect(p2.getByTestId('opponent-presence-dot')).toHaveAttribute('data-online', 'true', {
      timeout: 10_000,
    })

    await p1Ctx.close()
    await p2Ctx.close()
  })

  test('opponent presence goes offline when opponent disconnects during game', async ({
    browser,
  }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupAndStartGame(p1, p2, pair.p1Id)

    await expect(p1.getByTestId('opponent-presence-dot')).toHaveAttribute('data-online', 'true', {
      timeout: 10_000,
    })

    // P2 closes their context — simulates disconnect.
    await p2Ctx.close()

    await expect(p1.getByTestId('opponent-presence-dot')).toHaveAttribute('data-online', 'false', {
      timeout: 20_000,
    })

    await p1Ctx.close()
  })

  test('opponent presence text updates correctly', async ({ browser }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const { p1Ctx, p1, p2Ctx, p2 } = await createPlayerContexts(browser, pair)

    await setupAndStartGame(p1, p2, pair.p1Id)

    await expect(p1.getByTestId('opponent-presence-text')).toContainText('Opponent online', {
      timeout: 10_000,
    })

    // P2 disconnects.
    await p2Ctx.close()

    await expect(p1.getByTestId('opponent-presence-text')).toContainText('Opponent offline', {
      timeout: 30_000,
    })

    await p1Ctx.close()
  })
})
