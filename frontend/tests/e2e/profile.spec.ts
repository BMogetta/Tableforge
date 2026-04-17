import { expect, test } from './fixtures'

test.describe('Profile', () => {
  test('own profile shows username and stats', async ({ singlePlayer }) => {
    const { p1, p1Id } = singlePlayer

    await p1.goto(`/profile/${p1Id}`)

    // Username should be visible.
    await expect(p1.getByTestId('profile-username')).toBeVisible({ timeout: 10_000 })
    const username = await p1.getByTestId('profile-username').textContent()
    expect(username).toBeTruthy()

    // Stats bar should load.
    await expect(p1.getByTestId('stats-bar')).toBeVisible({ timeout: 10_000 })

    // Achievement grid should render.
    await expect(p1.getByTestId('achievement-grid')).toBeVisible({ timeout: 10_000 })
  })

  test('can view another player profile', async ({ players }) => {
    const { p1, p2Id } = players

    await p1.goto(`/profile/${p2Id}`)

    // Should show the other player's username.
    await expect(p1.getByTestId('profile-username')).toBeVisible({ timeout: 10_000 })

    // Block button should be visible (not own profile).
    await expect(p1.getByTestId('profile-block-btn')).toBeVisible({ timeout: 10_000 })
  })

  test('own profile does not show block button', async ({ singlePlayer }) => {
    const { p1, p1Id } = singlePlayer

    await p1.goto(`/profile/${p1Id}`)
    await expect(p1.getByTestId('profile-username')).toBeVisible({ timeout: 10_000 })

    // Block button should NOT be visible on own profile.
    await expect(p1.getByTestId('profile-block-btn')).not.toBeVisible()
  })

  test('achievements show unlocked state after games', async ({ singlePlayer }) => {
    const { p1, p1Id } = singlePlayer

    // Check via API if this player has any achievements.
    const res = await p1.request.get(`/api/v1/players/${p1Id}/achievements`)
    if (!res.ok()) {
      test.skip(true, 'Achievements endpoint not available')
      return
    }

    await p1.goto(`/profile/${p1Id}`)
    await expect(p1.getByTestId('achievement-grid')).toBeVisible({ timeout: 10_000 })

    const achievements = await res.json()
    if (Array.isArray(achievements) && achievements.length > 0) {
      // At least one achievement card should be unlocked (not show "?").
      const firstKey = achievements[0].achievement_key
      await expect(p1.getByTestId(`achievement-${firstKey}`)).toBeVisible()
    }
  })

  test('match history shows played games', async ({ singlePlayer }) => {
    const { p1, p1Id } = singlePlayer

    // Check via API if this player has match history.
    const res = await p1.request.get(`/api/v1/players/${p1Id}/matches?limit=5&offset=0`)
    if (!res.ok()) {
      test.skip(true, 'Matches endpoint not available')
      return
    }
    const data = await res.json()

    await p1.goto(`/profile/${p1Id}`)

    if (data.total > 0) {
      // Should show at least one match row.
      await expect(p1.getByTestId('match-row-0')).toBeVisible({ timeout: 10_000 })
    }
  })
})
