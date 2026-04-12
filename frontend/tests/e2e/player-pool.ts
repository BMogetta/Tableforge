/**
 * Dynamic player pool for e2e tests.
 *
 * Each test acquires N players from a shared pool (via a lock file) and releases
 * them on teardown. This allows full parallelism — no static pair assignments.
 *
 * The pool state is stored in `.player-pool.json` next to `.players.json`.
 * File locking is done via `.player-pool.lock` with retry+backoff to handle
 * concurrent access from multiple Playwright workers.
 */
import fs from 'fs'
import path from 'path'
import { fileURLToPath } from 'url'

const __dirname = path.dirname(fileURLToPath(import.meta.url))
const PLAYERS_FILE = path.join(__dirname, '.players.json')
const POOL_FILE = path.join(__dirname, '.player-pool.json')
const LOCK_FILE = path.join(__dirname, '.player-pool.lock')

export interface PoolPlayer {
  index: number // 1-based index matching player{N}_id
  id: string // UUID
  statePath: string // path to .auth/playerN.json
}

interface PoolState {
  locked: Record<string, string> // player index → test ID that holds it
}

// --- Lock helpers -----------------------------------------------------------

const LOCK_TIMEOUT_MS = 30_000
const LOCK_RETRY_MS = 50

function sleepSync(ms: number): void {
  const end = Date.now() + ms
  while (Date.now() < end) {
    /* busy-wait — Playwright workers are separate processes, keeps it simple */
  }
}

function acquireLock(): void {
  const deadline = Date.now() + LOCK_TIMEOUT_MS
  while (Date.now() < deadline) {
    try {
      fs.writeFileSync(LOCK_FILE, String(process.pid), { flag: 'wx' })
      return
    } catch {
      sleepSync(LOCK_RETRY_MS + Math.random() * LOCK_RETRY_MS)
    }
  }
  // Stale lock — force acquire
  fs.writeFileSync(LOCK_FILE, String(process.pid))
}

function releaseLock(): void {
  try {
    fs.unlinkSync(LOCK_FILE)
  } catch {
    // Already released
  }
}

// --- Pool state -------------------------------------------------------------

function readPool(): PoolState {
  try {
    return JSON.parse(fs.readFileSync(POOL_FILE, 'utf-8'))
  } catch {
    return { locked: {} }
  }
}

function writePool(state: PoolState): void {
  fs.writeFileSync(POOL_FILE, JSON.stringify(state))
}

function readPlayers(): Record<string, string> {
  return JSON.parse(fs.readFileSync(PLAYERS_FILE, 'utf-8'))
}

// --- Public API -------------------------------------------------------------

/**
 * Indices reserved for ranked matchmaking tests. The general pool never hands
 * these out, and only the `rankedPlayers` fixture acquires them — this
 * prevents cross-spec interference (another worker enqueuing/dequeuing the
 * same player while the ranked ticker is matching).
 */
export const RANKED_RESERVED_INDICES = [29, 30]

function toPoolPlayer(idx: number, players: Record<string, string>): PoolPlayer {
  return {
    index: idx,
    id: players[`player${idx}_id`],
    statePath: path.join(__dirname, `.auth/player${idx}.json`),
  }
}

/**
 * Acquire `count` players from the pool. Blocks until enough are available.
 * Returns an array of PoolPlayer objects. Skips reserved indices.
 */
export function acquirePlayers(count: number, testId: string): PoolPlayer[] {
  const players = readPlayers()
  const totalPlayers = Object.keys(players).length
  const reserved = new Set(RANKED_RESERVED_INDICES)

  const deadline = Date.now() + 30_000
  while (Date.now() < deadline) {
    acquireLock()
    try {
      const state = readPool()
      const available: number[] = []

      for (let i = 1; i <= totalPlayers; i++) {
        if (reserved.has(i)) continue
        if (!state.locked[String(i)]) {
          available.push(i)
        }
        if (available.length === count) break
      }

      if (available.length === count) {
        const result: PoolPlayer[] = available.map(idx => {
          state.locked[String(idx)] = testId
          return toPoolPlayer(idx, players)
        })
        writePool(state)
        return result
      }
    } finally {
      releaseLock()
    }

    // Not enough available — wait and retry
    sleepSync(200 + Math.random() * 100)
  }

  throw new Error(`Timed out waiting for ${count} available players from pool`)
}

/**
 * Acquire specific player indices from the pool, bypassing the reserved
 * filter. Used by fixtures that need dedicated slots (e.g. ranked).
 * Blocks if any requested index is held by another test.
 */
export function acquireSpecificPlayers(indices: number[], testId: string): PoolPlayer[] {
  const players = readPlayers()
  const totalPlayers = Object.keys(players).length

  for (const idx of indices) {
    if (idx < 1 || idx > totalPlayers) {
      throw new Error(`Player index ${idx} out of range (pool has ${totalPlayers})`)
    }
  }

  const deadline = Date.now() + 30_000
  while (Date.now() < deadline) {
    acquireLock()
    try {
      const state = readPool()
      const allFree = indices.every(idx => !state.locked[String(idx)])

      if (allFree) {
        for (const idx of indices) {
          state.locked[String(idx)] = testId
        }
        writePool(state)
        return indices.map(idx => toPoolPlayer(idx, players))
      }
    } finally {
      releaseLock()
    }

    sleepSync(200 + Math.random() * 100)
  }

  throw new Error(`Timed out waiting for specific players [${indices.join(',')}] from pool`)
}

/**
 * Release players back to the pool. Only releases players still held by `testId`
 * to prevent double-release from racing teardowns.
 */
export function releasePlayers(players: PoolPlayer[], testId?: string): void {
  acquireLock()
  try {
    const state = readPool()
    for (const p of players) {
      const key = String(p.index)
      if (!testId || state.locked[key] === testId) {
        delete state.locked[key]
      }
    }
    writePool(state)
  } finally {
    releaseLock()
  }
}

/**
 * Reset pool state — call in globalSetup or before test run.
 */
export function resetPool(): void {
  writePool({ locked: {} })
  try {
    fs.unlinkSync(LOCK_FILE)
  } catch {
    // OK
  }
}
