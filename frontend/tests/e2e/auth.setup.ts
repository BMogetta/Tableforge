import { test as setup, expect } from '@playwright/test'
import path from 'path'
import fs from 'fs'
import { fileURLToPath } from 'url'
const __dirname = path.dirname(fileURLToPath(import.meta.url))

// These player IDs must exist in the test database.
// Run the test seeder to create them before running Playwright.
const PLAYER1_ID = process.env.TEST_PLAYER1_ID!
const PLAYER2_ID = process.env.TEST_PLAYER2_ID!
const PLAYER3_ID = process.env.TEST_PLAYER3_ID!

const AUTH_DIR = path.join(__dirname, '.auth')

setup.beforeAll(() => {
  fs.mkdirSync(AUTH_DIR, { recursive: true })
})

// Creates an authenticated browser context for player 1.
setup('authenticate player 1', async ({ browser }) => {
  const context = await browser.newContext()
  const page = await context.newPage()

  // Hit the test-only login endpoint — sets the session cookie directly.
  const response = await page.request.get(
    `http://localhost/auth/test-login?player_id=${PLAYER1_ID}`
  )
  expect(response.status()).toBe(204)

  // Save the storage state (cookies) so tests can reuse it.
  await context.storageState({ path: path.join(AUTH_DIR, 'player1.json') })
  await context.close()
})

setup('authenticate player 2', async ({ browser }) => {
  const context = await browser.newContext()
  const page = await context.newPage()

  const response = await page.request.get(
    `http://localhost/auth/test-login?player_id=${PLAYER2_ID}`
  )
  expect(response.status()).toBe(204)

  await context.storageState({ path: path.join(AUTH_DIR, 'player2.json') })
  await context.close()
})

// player3 is used as a spectator — joins rooms without a seat.
setup('authenticate player 3', async ({ browser }) => {
  const context = await browser.newContext()
  const page = await context.newPage()

  const response = await page.request.get(
    `http://localhost/auth/test-login?player_id=${PLAYER3_ID}`
  )
  expect(response.status()).toBe(204)

  await context.storageState({ path: path.join(AUTH_DIR, 'player3.json') })
  await context.close()
})