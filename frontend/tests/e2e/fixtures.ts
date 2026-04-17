/**
 * Custom Playwright fixtures for dynamic player pool.
 *
 * Usage in spec files:
 *
 *   import { test, expect } from './fixtures'
 *
 *   test('my test', async ({ players }) => {
 *     const { p1, p2, p1Id, p2Id, cleanup } = players
 *     // ... use p1, p2 as Page objects
 *   })
 *
 *   test('spectator test', async ({ playersWithSpectator }) => {
 *     const { p1, p2, p3, p1Id, p2Id, p3Id, cleanup } = playersWithSpectator
 *   })
 */
import { type BrowserContext, test as base, expect, type Page } from '@playwright/test'
import {
  ADMIN_RESERVED_INDEX,
  acquirePlayers,
  acquireSpecificPlayers,
  type PoolPlayer,
  RANKED_RESERVED_INDICES,
  releasePlayers,
} from './player-pool'

export { expect }

// --- Cleanup helper ----------------------------------------------------------

export async function cleanupPlayer(page: Page, playerId: string) {
  // Reset all queue state (queue entry, bans, decline history).
  await page.request.delete(`/api/v1/queue/players/${playerId}/state`).catch(() => {})

  // Surrender any active sessions
  const sessRes = await page.request.get(`/api/v1/players/${playerId}/sessions`)
  if (sessRes.ok()) {
    const sessions: { id: string; room_id: string }[] = await sessRes.json()
    for (const session of sessions) {
      await page.request.post(`/api/v1/sessions/${session.id}/surrender`, {
        data: { player_id: playerId },
      })
      await page.request.post(`/api/v1/rooms/${session.room_id}/leave`, {
        data: { player_id: playerId },
      })
    }
  }
  // Leave any waiting rooms (no session yet — not covered above)
  const roomsRes = await page.request.get('/api/v1/rooms')
  if (roomsRes.ok()) {
    const body = await roomsRes.json()
    const items: { room: { id: string; status: string }; players: { id: string }[] }[] =
      body.items ?? []
    for (const item of items) {
      if (item.players.some(p => p.id === playerId) && item.room.status === 'waiting') {
        await page.request.post(`/api/v1/rooms/${item.room.id}/leave`, {
          data: {},
        })
      }
    }
  }
}

// --- Types ------------------------------------------------------------------

export interface Players2 {
  p1: Page
  p2: Page
  p1Ctx: BrowserContext
  p2Ctx: BrowserContext
  p1Id: string
  p2Id: string
}

export interface Players3 extends Players2 {
  p3: Page
  p3Ctx: BrowserContext
  p3Id: string
}

export interface Players1 {
  p1: Page
  p1Ctx: BrowserContext
  p1Id: string
}

// --- Fixtures ---------------------------------------------------------------

export const test = base.extend<{
  players: Players2
  playersWithSpectator: Players3
  singlePlayer: Players1
  rankedPlayers: Players2
  adminPlayer: Players1
}>({
  players: async ({ browser }, use, testInfo) => {
    const testId = `${testInfo.workerIndex}-${testInfo.testId}`
    const pool = acquirePlayers(2, testId)

    const p1Ctx = await browser.newContext({ storageState: pool[0].statePath })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))

    const p2Ctx = await browser.newContext({ storageState: pool[1].statePath })
    const p2 = await p2Ctx.newPage()
    p2.on('console', msg => console.log('P2:', msg.text()))
    p2.on('pageerror', err => console.log('P2 ERROR:', err.message))

    await Promise.all([cleanupPlayer(p1, pool[0].id), cleanupPlayer(p2, pool[1].id)])

    await p1.goto('/')
    await p2.goto('/')

    await use({
      p1,
      p2,
      p1Ctx,
      p2Ctx,
      p1Id: pool[0].id,
      p2Id: pool[1].id,
    })

    await p1Ctx.close().catch(() => {})
    await p2Ctx.close().catch(() => {})
    releasePlayers(pool, testId)
  },

  playersWithSpectator: async ({ browser }, use, testInfo) => {
    const testId = `${testInfo.workerIndex}-${testInfo.testId}`
    const pool = acquirePlayers(3, testId)

    const p1Ctx = await browser.newContext({ storageState: pool[0].statePath })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))

    const p2Ctx = await browser.newContext({ storageState: pool[1].statePath })
    const p2 = await p2Ctx.newPage()
    p2.on('console', msg => console.log('P2:', msg.text()))
    p2.on('pageerror', err => console.log('P2 ERROR:', err.message))

    const p3Ctx = await browser.newContext({ storageState: pool[2].statePath })
    const p3 = await p3Ctx.newPage()
    p3.on('console', msg => console.log('P3:', msg.text()))
    p3.on('pageerror', err => console.log('P3 ERROR:', err.message))

    await Promise.all([
      cleanupPlayer(p1, pool[0].id),
      cleanupPlayer(p2, pool[1].id),
      cleanupPlayer(p3, pool[2].id),
    ])

    await p1.goto('/')
    await p2.goto('/')
    await p3.goto('/')

    await use({
      p1,
      p2,
      p3,
      p1Ctx,
      p2Ctx,
      p3Ctx,
      p1Id: pool[0].id,
      p2Id: pool[1].id,
      p3Id: pool[2].id,
    })

    await p1Ctx.close().catch(() => {})
    await p2Ctx.close().catch(() => {})
    await p3Ctx.close().catch(() => {})
    releasePlayers(pool, testId)
  },

  /**
   * Dedicated 2-player slot for ranked matchmaking tests. Uses reserved pool
   * indices that no other fixture can hand out — prevents cross-spec
   * interference with the ranked queue/ticker.
   */
  rankedPlayers: async ({ browser }, use, testInfo) => {
    const testId = `${testInfo.workerIndex}-${testInfo.testId}`
    const pool = acquireSpecificPlayers(RANKED_RESERVED_INDICES, testId)

    const p1Ctx = await browser.newContext({ storageState: pool[0].statePath })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))

    const p2Ctx = await browser.newContext({ storageState: pool[1].statePath })
    const p2 = await p2Ctx.newPage()
    p2.on('console', msg => console.log('P2:', msg.text()))
    p2.on('pageerror', err => console.log('P2 ERROR:', err.message))

    await Promise.all([cleanupPlayer(p1, pool[0].id), cleanupPlayer(p2, pool[1].id)])

    await p1.goto('/')
    await p2.goto('/')

    await use({
      p1,
      p2,
      p1Ctx,
      p2Ctx,
      p1Id: pool[0].id,
      p2Id: pool[1].id,
    })

    await p1Ctx.close().catch(() => {})
    await p2Ctx.close().catch(() => {})
    releasePlayers(pool, testId)
  },

  /**
   * Dedicated admin (manager-role) player for admin panel tests.
   * seed-test promotes this pool slot to role='manager'.
   */
  adminPlayer: async ({ browser }, use, testInfo) => {
    const testId = `${testInfo.workerIndex}-${testInfo.testId}`
    const pool = acquireSpecificPlayers([ADMIN_RESERVED_INDEX], testId)

    const p1Ctx = await browser.newContext({ storageState: pool[0].statePath })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('Admin:', msg.text()))
    p1.on('pageerror', err => console.log('Admin ERROR:', err.message))

    await cleanupPlayer(p1, pool[0].id)
    await p1.goto('/')

    await use({
      p1,
      p1Ctx,
      p1Id: pool[0].id,
    })

    await p1Ctx.close().catch(() => {})
    releasePlayers(pool, testId)
  },

  singlePlayer: async ({ browser }, use, testInfo) => {
    const testId = `${testInfo.workerIndex}-${testInfo.testId}`
    const pool = acquirePlayers(1, testId)

    const p1Ctx = await browser.newContext({ storageState: pool[0].statePath })
    const p1 = await p1Ctx.newPage()
    p1.on('console', msg => console.log('P1:', msg.text()))
    p1.on('pageerror', err => console.log('P1 ERROR:', err.message))

    await cleanupPlayer(p1, pool[0].id)
    await p1.goto('/')

    await use({
      p1,
      p1Ctx,
      p1Id: pool[0].id,
    })

    await p1Ctx.close().catch(() => {})
    releasePlayers(pool, testId)
  },
})
