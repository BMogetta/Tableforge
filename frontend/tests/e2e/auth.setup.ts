import { test as setup, expect } from '@playwright/test'
import path from 'path'
import fs from 'fs'
import { fileURLToPath } from 'url'
import { resetPool } from './player-pool'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const AUTH_DIR = path.join(__dirname, '.auth')
const PLAYERS_FILE = path.join(__dirname, '.players.json')

// Read all player IDs from the seed-test output.
const players: Record<string, string> = JSON.parse(fs.readFileSync(PLAYERS_FILE, 'utf-8'))
const playerCount = Object.keys(players).length

// Authenticate all players in a single serial test to avoid rate limiting.
setup('authenticate all players', async ({ browser }) => {
  fs.mkdirSync(AUTH_DIR, { recursive: true })
  resetPool()

  for (let i = 1; i <= playerCount; i++) {
    const playerId = players[`player${i}_id`]
    const context = await browser.newContext()
    const page = await context.newPage()

    const response = await page.request.get(
      `http://localhost/auth/test-login?player_id=${playerId}`,
    )
    expect(response.status()).toBe(204)

    await context.storageState({ path: path.join(AUTH_DIR, `player${i}.json`) })
    await context.close()
  }
})
