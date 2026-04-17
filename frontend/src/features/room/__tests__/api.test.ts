import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { bots, rooms } from '../api'

// Stub telemetry
vi.mock('@/lib/telemetry', () => ({
  tracer: {
    startActiveSpan: (_name: string, _opts: unknown, fn: (span: unknown) => unknown) =>
      fn({
        setAttributes: vi.fn(),
        setAttribute: vi.fn(),
        setStatus: vi.fn(),
        recordException: vi.fn(),
        end: vi.fn(),
      }),
  },
  emitErrorLog: vi.fn(),
}))

vi.mock('@opentelemetry/api', () => ({
  SpanKind: { CLIENT: 1 },
  SpanStatusCode: { OK: 0, ERROR: 1 },
  context: { active: vi.fn() },
  propagation: { inject: vi.fn() },
}))

// --- Helpers -----------------------------------------------------------------

let mockFetch: ReturnType<typeof vi.fn>

function jsonResponse(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'content-type': 'application/json' },
  })
}

function lastFetchCall() {
  const [url, init] = mockFetch.mock.calls[0]
  return { url: url as string, method: init?.method ?? 'GET', body: init?.body }
}

const now = new Date().toISOString()
const validRoom = {
  room: {
    id: 'r1',
    code: 'ABCD',
    game_id: 'tictactoe',
    owner_id: 'p1',
    status: 'waiting',
    max_players: 2,
    created_at: now,
    updated_at: now,
  },
  players: [
    {
      id: 'p1',
      username: 'alice',
      seat: 0,
      joined_at: now,
      is_bot: false,
      role: 'player',
      avatar_url: '',
      created_at: now,
    },
  ],
  settings: {},
}

const validSession = {
  id: 's1',
  room_id: 'r1',
  game_id: 'tictactoe',
  mode: 'casual',
  move_count: 0,
  suspend_count: 0,
  ready_players: [],
  last_move_at: new Date().toISOString(),
  started_at: new Date().toISOString(),
}

beforeEach(() => {
  mockFetch = vi.fn()
  vi.stubGlobal('fetch', mockFetch)
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

// --- Rooms -------------------------------------------------------------------

describe('rooms API', () => {
  it('list fetches with pagination', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ items: [], total: 0 }))

    await rooms.list(10, 5)
    expect(lastFetchCall().url).toContain('/rooms?limit=10&offset=5')
  })

  it('get fetches by id', async () => {
    mockFetch.mockResolvedValue(jsonResponse(validRoom))

    await rooms.get('r1')
    expect(lastFetchCall().url).toContain('/rooms/r1')
  })

  it('create sends POST with game_id', async () => {
    mockFetch.mockResolvedValue(jsonResponse(validRoom))

    await rooms.create('tictactoe', 'p1')
    const call = lastFetchCall()
    expect(call.method).toBe('POST')
    expect(JSON.parse(call.body)).toEqual({ game_id: 'tictactoe' })
  })

  it('create includes turn_timeout_secs when provided', async () => {
    mockFetch.mockResolvedValue(jsonResponse(validRoom))

    await rooms.create('tictactoe', 'p1', 30)
    expect(JSON.parse(lastFetchCall().body)).toEqual({
      game_id: 'tictactoe',
      turn_timeout_secs: 30,
    })
  })

  it('join sends POST with code', async () => {
    mockFetch.mockResolvedValue(jsonResponse(validRoom))

    await rooms.join('ABCD')
    const call = lastFetchCall()
    expect(call.method).toBe('POST')
    expect(call.url).toContain('/rooms/join')
    expect(JSON.parse(call.body)).toEqual({ code: 'ABCD' })
  })

  it('leave sends POST', async () => {
    mockFetch.mockResolvedValue(
      new Response(null, { status: 204, headers: { 'content-length': '0' } }),
    )

    await rooms.leave('r1')
    const call = lastFetchCall()
    expect(call.url).toContain('/rooms/r1/leave')
    expect(call.method).toBe('POST')
  })

  it('start sends POST', async () => {
    mockFetch.mockResolvedValue(jsonResponse(validSession))

    await rooms.start('r1')
    const call = lastFetchCall()
    expect(call.url).toContain('/rooms/r1/start')
    expect(call.method).toBe('POST')
  })

  it('updateSetting sends PUT with value', async () => {
    mockFetch.mockResolvedValue(
      new Response(null, { status: 204, headers: { 'content-length': '0' } }),
    )

    await rooms.updateSetting('r1', 'turn_timeout', '60')
    const call = lastFetchCall()
    expect(call.url).toContain('/rooms/r1/settings/turn_timeout')
    expect(call.method).toBe('PUT')
    expect(JSON.parse(call.body)).toEqual({ value: '60' })
  })

  it('updateSetting encodes key', async () => {
    mockFetch.mockResolvedValue(
      new Response(null, { status: 204, headers: { 'content-length': '0' } }),
    )

    await rooms.updateSetting('r1', 'key with spaces', 'val')
    expect(lastFetchCall().url).toContain('key%20with%20spaces')
  })
})

// --- Bots --------------------------------------------------------------------

describe('bots API', () => {
  it('profiles fetches GET', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]))

    await bots.profiles()
    expect(lastFetchCall().url).toContain('/bots/profiles')
  })

  it('add sends POST with profile', async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ id: 'bot1', username: 'bot', is_bot: true, role: 'player', created_at: now }),
    )

    await bots.add('r1', 'p1', 'easy')
    const call = lastFetchCall()
    expect(call.url).toContain('/rooms/r1/bots')
    expect(call.method).toBe('POST')
    expect(JSON.parse(call.body)).toEqual({ profile: 'easy' })
  })

  it('remove sends DELETE', async () => {
    mockFetch.mockResolvedValue(
      new Response(null, { status: 204, headers: { 'content-length': '0' } }),
    )

    await bots.remove('r1', 'p1', 'bot1')
    const call = lastFetchCall()
    expect(call.url).toContain('/rooms/r1/bots/bot1')
    expect(call.method).toBe('DELETE')
  })
})
