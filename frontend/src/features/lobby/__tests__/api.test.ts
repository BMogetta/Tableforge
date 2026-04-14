import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { leaderboard, gameRegistry, queue } from '../api'

vi.mock('@/lib/telemetry', () => ({
  tracer: {
    startActiveSpan: (_n: string, _o: unknown, fn: (s: unknown) => unknown) =>
      fn({ setAttributes: vi.fn(), setAttribute: vi.fn(), setStatus: vi.fn(), recordException: vi.fn(), end: vi.fn() }),
  },
  emitErrorLog: vi.fn(),
}))
vi.mock('@opentelemetry/api', () => ({
  SpanKind: { CLIENT: 1 }, SpanStatusCode: { OK: 0, ERROR: 1 },
  context: { active: vi.fn() }, propagation: { inject: vi.fn() },
}))

let mockFetch: ReturnType<typeof vi.fn>

function json(body: unknown, status = 200) {
  return new Response(JSON.stringify(body), { status, headers: { 'content-type': 'application/json' } })
}
function lastCall() {
  const [url, init] = mockFetch.mock.calls[0]
  return { url: url as string, method: init?.method ?? 'GET', body: init?.body }
}

beforeEach(() => { mockFetch = vi.fn(); vi.stubGlobal('fetch', mockFetch) })
afterEach(() => { vi.restoreAllMocks(); vi.unstubAllGlobals() })

describe('leaderboard', () => {
  it('get fetches with game_id and limit', async () => {
    mockFetch.mockResolvedValue(json({ game_id: 'tictactoe', entries: [{ rank: 1, player_id: 'p1', username: 'alice', is_bot: false, display_rating: 1500, games_played: 10 }], total: 1 }))
    const entries = await leaderboard.get('tictactoe', 5)
    expect(lastCall().url).toContain('/ratings/tictactoe/leaderboard?limit=5')
    expect(entries).toHaveLength(1)
    expect(entries[0].username).toBe('alice')
  })

  it('get appends include_bots=true when requested', async () => {
    mockFetch.mockResolvedValue(json({ game_id: 'tictactoe', entries: [], total: 0 }))
    await leaderboard.get('tictactoe', 5, true)
    expect(lastCall().url).toContain('include_bots=true')
  })
})

describe('gameRegistry', () => {
  it('list fetches /games', async () => {
    mockFetch.mockResolvedValue(json([{ id: 'tictactoe', name: 'Tic Tac Toe', min_players: 2, max_players: 2, settings: [] }]))
    await gameRegistry.list()
    expect(lastCall().url).toContain('/games')
  })
})

describe('queue', () => {
  it('join sends POST with player_id', async () => {
    mockFetch.mockResolvedValue(json({ position: 1, estimated_wait_secs: 30 }))
    await queue.join('p1')
    const c = lastCall()
    expect(c.method).toBe('POST')
    expect(c.url).toContain('/queue')
    expect(JSON.parse(c.body)).toEqual({ player_id: 'p1' })
  })

  it('leave sends DELETE', async () => {
    mockFetch.mockResolvedValue(new Response(null, { status: 204, headers: { 'content-length': '0' } }))
    await queue.leave('p1')
    expect(lastCall().method).toBe('DELETE')
  })

  it('accept sends POST with match_id', async () => {
    mockFetch.mockResolvedValue(new Response(null, { status: 204, headers: { 'content-length': '0' } }))
    await queue.accept('p1', 'match-1')
    const c = lastCall()
    expect(c.url).toContain('/queue/accept')
    expect(JSON.parse(c.body)).toEqual({ match_id: 'match-1' })
  })

  it('decline sends POST with match_id', async () => {
    mockFetch.mockResolvedValue(new Response(null, { status: 204, headers: { 'content-length': '0' } }))
    await queue.decline('p1', 'match-1')
    const c = lastCall()
    expect(c.url).toContain('/queue/decline')
    expect(JSON.parse(c.body)).toEqual({ match_id: 'match-1' })
  })
})
