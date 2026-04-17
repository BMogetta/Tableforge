import { expect, test } from './fixtures'

test.describe('Auth and access control', () => {
  test('error boundary catches render errors and shows recovery UI', async ({ singlePlayer }) => {
    const { p1 } = singlePlayer

    await p1.goto('/test/error')

    await expect(p1.getByText('SOMETHING WENT WRONG')).toBeVisible({ timeout: 10_000 })
    await expect(p1.getByText('Test error triggered intentionally')).toBeVisible()

    await expect(p1.getByRole('button', { name: 'Try Again' })).toBeVisible()
    await expect(p1.getByRole('button', { name: 'Back to Lobby' })).toBeVisible()
  })
})
