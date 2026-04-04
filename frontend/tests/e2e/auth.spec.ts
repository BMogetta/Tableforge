import { test, expect } from '@playwright/test'
import { getPair } from './helpers'

test.describe('Auth and access control', () => {
  test('error boundary catches render errors and shows recovery UI', async ({
    browser,
  }, testInfo) => {
    const pair = getPair(testInfo.project.name)
    const ctx = await browser.newContext({ storageState: pair.p1State })
    const page = await ctx.newPage()

    await page.goto('/test/error')

    await expect(page.getByText('SOMETHING WENT WRONG')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Test error triggered intentionally')).toBeVisible()

    await expect(page.getByRole('button', { name: 'Try Again' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Go to Lobby' })).toBeVisible()

    await ctx.close()
  })
})
