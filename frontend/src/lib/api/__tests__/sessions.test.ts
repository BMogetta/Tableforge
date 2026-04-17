import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { sessions } from '../sessions'

// Stub telemetry
vi.mock('../../telemetry', () => ({
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

beforeEach(() => {
  mockFetch = vi.fn()
  vi.stubGlobal('fetch', mockFetch)
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

// --- Tests -------------------------------------------------------------------

describe('sessions API client', () => {
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

  it('get fetches correct URL', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ session: validSession, state: {}, result: null }))

    await sessions.get('session-42')
    expect(lastFetchCall().url).toContain('/sessions/session-42')
    expect(lastFetchCall().method).toBe('GET')
  })

  it('ready sends POST with empty body', async () => {
    mockFetch.mockResolvedValue(
      jsonResponse({ all_ready: true, ready_players: ['p1'], required: 1 }),
    )

    await sessions.ready('s1')
    const call = lastFetchCall()
    expect(call.url).toContain('/sessions/s1/ready')
    expect(call.method).toBe('POST')
    expect(call.body).toBe(JSON.stringify({}))
  })

  it('move sends POST with payload', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ move_number: 1, is_over: false }))

    await sessions.move('s1', { cell: 4 })
    const call = lastFetchCall()
    expect(call.url).toContain('/sessions/s1/move')
    expect(call.method).toBe('POST')
    expect(JSON.parse(call.body)).toEqual({ payload: { cell: 4 } })
  })

  it('surrender sends POST with empty body', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ move_number: 1, is_over: true }))

    await sessions.surrender('s1')
    const call = lastFetchCall()
    expect(call.url).toContain('/sessions/s1/surrender')
    expect(call.method).toBe('POST')
    expect(call.body).toBe(JSON.stringify({}))
  })

  it('rematch sends POST with empty body', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ votes: 1, total_players: 2 }))

    await sessions.rematch('s1')
    const call = lastFetchCall()
    expect(call.url).toContain('/sessions/s1/rematch')
    expect(call.method).toBe('POST')
  })

  it('pause sends POST with empty body', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ all_voted: false, votes: 1, required: 2 }))

    await sessions.pause('s1')
    const call = lastFetchCall()
    expect(call.url).toContain('/sessions/s1/pause')
    expect(call.method).toBe('POST')
  })

  it('resume sends POST with empty body', async () => {
    mockFetch.mockResolvedValue(jsonResponse({ all_voted: true, votes: 2, required: 2 }))

    const result = await sessions.resume('s1')
    expect(lastFetchCall().url).toContain('/sessions/s1/resume')
    expect(result.all_voted).toBe(true)
  })

  it('forceClose sends DELETE', async () => {
    mockFetch.mockResolvedValue(
      new Response(null, { status: 204, headers: { 'content-length': '0' } }),
    )

    await sessions.forceClose('s1')
    const call = lastFetchCall()
    expect(call.url).toContain('/sessions/s1')
    expect(call.method).toBe('DELETE')
  })

  it('events fetches GET', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]))

    await sessions.events('s1')
    expect(lastFetchCall().url).toContain('/sessions/s1/events')
  })

  it('history fetches GET', async () => {
    mockFetch.mockResolvedValue(jsonResponse([]))

    await sessions.history('s1')
    expect(lastFetchCall().url).toContain('/sessions/s1/history')
  })
})
