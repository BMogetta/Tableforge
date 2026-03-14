/// <reference types="node" />
import { defineConfig, devices } from '@playwright/test'

export default defineConfig({
  testDir: './tests/e2e',
  fullyParallel: false, // game tests must run sequentially — shared server state
  forbidOnly: !!process.env.CI,
  retries: process.env.CI ? 1 : 0,
  workers: 2,
  reporter: 'html',
  timeout: 60_000,

  use: {
    baseURL: 'http://localhost',
    // Cookies are required for auth — don't clear between tests.
    // Each test uses storageState to load pre-authenticated sessions.
    trace: 'on-first-retry',
    screenshot: 'only-on-failure',
  },

  projects: [
    { name: 'setup', testMatch: /.*\.setup\.ts/ },
    {
      name: 'game-tests',
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup'],
      testMatch: /\/(game|lobby)\.spec\.ts/,
    },
    {
      name: 'chat-tests',
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup'],
      testMatch: /\/chat\.spec\.ts/,
    },
    {
      name: 'settings-tests',
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup'],
      testMatch: /\/settings\.spec\.ts/,
    },
    {
      name: 'spectator-tests',
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup'],
      testMatch: /\/spectator\.spec\.ts/,
    },
    {
      name: 'leaderboard-tests',
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup', 'game-tests'],
      testMatch: /\/leaderboard\.spec\.ts/,
    },
    {
      name: 'session-history-tests',
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup', 'game-tests'],
      testMatch: /\/session-history\.spec\.ts/,
    },
    {
      name: 'presence-tests',
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup'],
      testMatch: /\/presence\.spec\.ts/,
    },
    {
      name: 'chromium-parallel',
      use: { ...devices['Desktop Chrome'] },
      dependencies: ['setup'],
      testMatch: /\/auth\.spec\.ts/,
      fullyParallel: true,
    },
  ],
})