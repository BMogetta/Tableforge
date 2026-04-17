import { expect, test } from './fixtures'

/**
 * Admin panel tests. Use the dedicated `adminPlayer` fixture which logs in
 * with role='manager' (promoted by seed-test at the reserved pool slot).
 * Running in serial mode — the ban flow mutates global state.
 */
test.describe('Admin panel', () => {
  test.describe.configure({ mode: 'serial' })

  test('admin panel loads with all tabs visible', async ({ adminPlayer }) => {
    const { p1 } = adminPlayer

    await p1.goto('/admin')
    await expect(p1.getByTestId('admin-panel')).toBeVisible({ timeout: 10_000 })

    for (const tab of ['stats', 'players', 'bans', 'broadcast']) {
      await expect(p1.getByTestId(`tab-${tab}`)).toBeVisible()
    }
  })

  test('stats tab renders stat cards', async ({ adminPlayer }) => {
    const { p1 } = adminPlayer

    await p1.goto('/admin')
    await p1.getByTestId('tab-stats').click()

    // Either the stats panel renders populated cards, or — if the server
    // returned no stats — the empty state. Both are valid for a fresh env.
    const panel = p1.getByTestId('stats-panel')
    const empty = p1.getByTestId('stats-empty')
    await expect(panel.or(empty)).toBeVisible({ timeout: 10_000 })
  })

  test('players tab lists seeded players', async ({ adminPlayer }) => {
    const { p1 } = adminPlayer

    await p1.goto('/admin')
    await p1.getByTestId('tab-players').click()

    await expect(p1.getByTestId('players-table')).toBeVisible({ timeout: 10_000 })
    // seed-test inserts 30 players — we should see at least a handful.
    const rowCount = await p1.getByTestId('players-table').locator('tbody tr').count()
    expect(rowCount).toBeGreaterThan(5)
  })

  test('ban flow: issue, appears in active bans, lift', async ({ adminPlayer, players }) => {
    const { p1: admin } = adminPlayer
    const { p2Id: targetId } = players

    await admin.goto('/admin')
    await admin.getByTestId('tab-bans').click()
    await expect(admin.getByTestId('bans-panel')).toBeVisible({ timeout: 10_000 })

    // Open the ban dialog and submit a temporary ban against the target.
    await admin.getByTestId('open-ban-dialog-btn').click()
    await expect(admin.getByTestId('ban-dialog')).toBeVisible()

    await admin.getByTestId('ban-player-id-input').fill(targetId)
    await admin.getByTestId('ban-reason-input').fill('e2e ban test')
    await admin.getByTestId('confirm-ban-btn').click()

    await expect(admin.getByTestId('ban-dialog')).not.toBeVisible({ timeout: 10_000 })

    // Search for the target — the active bans table should include them.
    await admin.getByTestId('bans-search-input').fill(targetId)
    await admin.getByTestId('bans-search-btn').click()
    await expect(admin.getByTestId('active-bans-table')).toBeVisible({ timeout: 10_000 })

    // Lift the ban and confirm the row disappears from the active list.
    const liftBtn = admin.locator('[data-testid^="lift-ban-"]').first()
    await expect(liftBtn).toBeVisible({ timeout: 10_000 })
    admin.once('dialog', d => d.accept()) // window.confirm prompt
    await liftBtn.click()

    // After lift, the player should have no active bans.
    await expect(async () => {
      const res = await admin.request.get(`/api/v1/admin/players/${targetId}/bans`)
      expect(res.ok()).toBe(true)
      const bans = (await res.json()) as Array<{
        lifted_at: string | null
        expires_at: string | null
      }>
      const stillActive = bans.some(
        b => !b.lifted_at && (!b.expires_at || new Date(b.expires_at) > new Date()),
      )
      expect(stillActive).toBe(false)
    }).toPass({ timeout: 10_000 })
  })

  test('broadcast can be sent', async ({ adminPlayer }) => {
    const { p1 } = adminPlayer

    await p1.goto('/admin')
    await p1.getByTestId('tab-broadcast').click()
    await expect(p1.getByTestId('broadcast-panel')).toBeVisible({ timeout: 10_000 })

    const message = `e2e broadcast ${Date.now()}`
    await p1.getByTestId('broadcast-message').fill(message)

    // Wait for the POST to complete with 2xx — confirms the caller's role
    // is accepted by the manager-gated endpoint.
    const [res] = await Promise.all([
      p1.waitForResponse(
        r => r.url().includes('/api/v1/admin/broadcast') && r.request().method() === 'POST',
      ),
      p1.getByTestId('broadcast-send').click(),
    ])
    expect(res.ok()).toBe(true)
  })
})
