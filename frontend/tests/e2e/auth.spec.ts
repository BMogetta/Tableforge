import { test, expect } from '@playwright/test'
import { fileURLToPath } from 'url'
import path from 'path'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const PLAYER1_STATE = path.join(__dirname, '.auth/player1.json')

test.describe('Auth and access control', () => {
  test('error boundary catches render errors and shows recovery UI', async ({ browser }) => {
    // /test/error renders a component that throws during render.
    // Only available when VITE_TEST_MODE=true (build arg set in docker-compose).
    const ctx = await browser.newContext({ storageState: PLAYER1_STATE })
    const page = await ctx.newPage()

    await page.goto('/test/error')

    // ErrorBoundary should catch the throw and show the error screen,
    // not a blank page or an unhandled exception.
    await expect(page.getByText('SOMETHING WENT WRONG')).toBeVisible({ timeout: 10_000 })
    await expect(page.getByText('Test error triggered intentionally')).toBeVisible()

    // The recovery buttons should be present and functional.
    await expect(page.getByRole('button', { name: 'Try Again' })).toBeVisible()
    await expect(page.getByRole('button', { name: 'Go to Lobby' })).toBeVisible()

    await ctx.close()
  })
})
