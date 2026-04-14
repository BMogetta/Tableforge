// MCP server exposing recess game-server operations as callable tools.
// Consumes the same HTTP API the frontend does, authenticated via the
// TEST_MODE-only /auth/test-login endpoint (cookie-based). Intended for
// local dev only — do not point it at production.

import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { McpServer } from '@modelcontextprotocol/sdk/server/mcp.js'
import { StdioServerTransport } from '@modelcontextprotocol/sdk/server/stdio.js'
import { z } from 'zod'

const BASE_URL = process.env.RECESS_BASE_URL ?? 'http://localhost'

// Resolve the player UUID in this order:
// 1. RECESS_PLAYER_ID env var (explicit override).
// 2. RECESS_PLAYER_KEY (e.g. "player1_id") looked up in the seed JSON.
// 3. The first player UUID in frontend/tests/e2e/.players.json (written by
//    `make seed-test`). Defaults to player1_id so zero config works after
//    seeding.
function resolvePlayerId(): string {
  if (process.env.RECESS_PLAYER_ID) return process.env.RECESS_PLAYER_ID

  // Walk up from this file to find the repo root (where .players.json lives).
  // The MCP is spawned from the repo via Claude Code, so cwd is usually the
  // repo root — but resolve relative to __dirname to be robust.
  const candidates = [
    resolve(process.cwd(), 'frontend/tests/e2e/.players.json'),
    resolve(import.meta.dirname, '../../../frontend/tests/e2e/.players.json'),
  ]
  let seedPath: string | null = null
  let raw: string | null = null
  for (const p of candidates) {
    try {
      raw = readFileSync(p, 'utf8')
      seedPath = p
      break
    } catch {
      /* try next */
    }
  }
  if (!raw) {
    console.error(
      'Could not resolve a player UUID. Either set RECESS_PLAYER_ID explicitly, or ' +
        `run \`make seed-test\` so ${candidates[0]} exists.`,
    )
    process.exit(1)
  }
  const seed = JSON.parse(raw) as Record<string, string>
  const key = process.env.RECESS_PLAYER_KEY ?? 'player1_id'
  const id = seed[key]
  if (!id) {
    console.error(`Seed file ${seedPath} has no key "${key}". Known keys: ${Object.keys(seed).join(', ')}`)
    process.exit(1)
  }
  return id
}

const PLAYER_ID = resolvePlayerId()

// ---------------------------------------------------------------------------
// Minimal cookie-backed HTTP client
// ---------------------------------------------------------------------------

let cookieHeader = ''

function captureSetCookie(res: Response) {
  // Collect every Set-Cookie; store just the name=value pairs for replay.
  const raw = res.headers.getSetCookie?.() ?? []
  if (raw.length === 0) return
  const pairs = raw.map(c => c.split(';')[0].trim()).filter(Boolean)
  if (pairs.length === 0) return
  const existing = new Map(
    cookieHeader
      .split(';')
      .map(s => s.trim())
      .filter(Boolean)
      .map(p => {
        const i = p.indexOf('=')
        return [p.slice(0, i), p.slice(i + 1)] as [string, string]
      }),
  )
  for (const p of pairs) {
    const i = p.indexOf('=')
    existing.set(p.slice(0, i), p.slice(i + 1))
  }
  cookieHeader = Array.from(existing, ([k, v]) => `${k}=${v}`).join('; ')
}

async function request<T = unknown>(
  path: string,
  init: RequestInit = {},
): Promise<{ status: number; body: T | string }> {
  const headers = new Headers(init.headers)
  if (cookieHeader) headers.set('cookie', cookieHeader)
  if (init.body && !headers.has('content-type')) {
    headers.set('content-type', 'application/json')
  }
  const res = await fetch(`${BASE_URL}${path}`, { ...init, headers })
  captureSetCookie(res)
  const text = await res.text()
  const contentType = res.headers.get('content-type') ?? ''
  const body = contentType.includes('application/json') && text ? JSON.parse(text) : text
  return { status: res.status, body: body as T | string }
}

async function login(): Promise<void> {
  const res = await request(`/auth/test-login?player_id=${PLAYER_ID}`)
  if (res.status !== 200 && res.status !== 204) {
    throw new Error(
      `test-login failed (${res.status}): ${typeof res.body === 'string' ? res.body : JSON.stringify(res.body)}. ` +
        'Ensure auth-service is running with TEST_MODE=true and RECESS_PLAYER_ID (or the seed fallback) is a seeded test player UUID.',
    )
  }
}

// Wraps an API call so failures become a structured tool result instead of
// throwing — MCP clients surface tool errors awkwardly.
function asTextResult(data: unknown): {
  content: Array<{ type: 'text'; text: string }>
} {
  return {
    content: [
      { type: 'text', text: typeof data === 'string' ? data : JSON.stringify(data, null, 2) },
    ],
  }
}

async function callApi(
  path: string,
  init: RequestInit = {},
): Promise<{ content: Array<{ type: 'text'; text: string }>; isError?: boolean }> {
  try {
    const res = await request(path, init)
    if (res.status >= 400) {
      return { ...asTextResult({ status: res.status, body: res.body }), isError: true }
    }
    return asTextResult(res.body)
  } catch (e) {
    return { ...asTextResult({ error: e instanceof Error ? e.message : String(e) }), isError: true }
  }
}

// ---------------------------------------------------------------------------
// MCP server
// ---------------------------------------------------------------------------

const server = new McpServer({ name: 'recess-game', version: '0.1.0' })

server.tool(
  'list_games',
  'Lists every game registered on the server (id, min/max players, etc.).',
  {},
  async () => callApi('/api/v1/games'),
)

server.tool(
  'list_scenarios',
  'Lists debug scenario fixtures available for a game. Requires the server to run with ALLOW_DEBUG_SCENARIOS=true (auto-true in dev).',
  { game_id: z.string().describe('e.g. "rootaccess"') },
  async ({ game_id }) => callApi(`/api/v1/games/${encodeURIComponent(game_id)}/scenarios`),
)

server.tool(
  'get_session',
  'Fetches a session by id — includes current state, players, phase, move count.',
  { session_id: z.string().describe('UUID of the session') },
  async ({ session_id }) => callApi(`/api/v1/sessions/${encodeURIComponent(session_id)}`),
)

server.tool(
  'load_scenario',
  'Applies a named scenario fixture to an active session. Cancels timers, broadcasts the new state to connected clients, and wakes the bot if the loaded state hands the turn to one.',
  {
    session_id: z.string(),
    scenario_id: z.string().describe('ID returned by list_scenarios (e.g. "firewall_standoff")'),
  },
  async ({ session_id, scenario_id }) =>
    callApi(`/api/v1/sessions/${encodeURIComponent(session_id)}/debug/load-scenario`, {
      method: 'POST',
      body: JSON.stringify({ scenario_id }),
    }),
)

server.tool(
  'load_state',
  'Replaces a session\'s state with raw JSON (escape hatch when no fixture fits). The value must parse as an engine.GameState — { current_player_id, data: {...} }.',
  {
    session_id: z.string(),
    state: z
      .record(z.unknown())
      .describe('Full engine.GameState payload — same shape as get_session().state'),
  },
  async ({ session_id, state }) =>
    callApi(`/api/v1/sessions/${encodeURIComponent(session_id)}/debug/load-state`, {
      method: 'POST',
      body: JSON.stringify({ state }),
    }),
)

server.tool(
  'apply_move',
  'Submits a move on behalf of the authenticated player. Payload shape is game-specific — for rootaccess, { card: string, target_player_id?: uuid, guess?: card }.',
  {
    session_id: z.string(),
    payload: z.record(z.unknown()).describe('Move payload — game-specific'),
  },
  async ({ session_id, payload }) =>
    callApi(`/api/v1/sessions/${encodeURIComponent(session_id)}/move`, {
      method: 'POST',
      body: JSON.stringify({ payload }),
    }),
)

server.tool(
  'surrender',
  'Surrenders a session on behalf of the authenticated player. Ends the game immediately.',
  { session_id: z.string() },
  async ({ session_id }) =>
    callApi(`/api/v1/sessions/${encodeURIComponent(session_id)}/surrender`, {
      method: 'POST',
      body: JSON.stringify({}),
    }),
)

server.tool(
  'list_player_sessions',
  'Lists active sessions for a player. Defaults to the authenticated player.',
  {
    player_id: z.string().optional().describe('Defaults to the authenticated player'),
  },
  async ({ player_id }) =>
    callApi(`/api/v1/players/${encodeURIComponent(player_id ?? PLAYER_ID)}/sessions`),
)

// ---------------------------------------------------------------------------
// Boot
// ---------------------------------------------------------------------------

await login()

const transport = new StdioServerTransport()
await server.connect(transport)
