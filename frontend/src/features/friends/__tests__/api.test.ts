import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { blocks, dmConversations, friends, players, presence } from '../api'

vi.mock('@/lib/telemetry', () => ({
  tracer: {
    startActiveSpan: (_n: string, _o: unknown, fn: (s: unknown) => unknown) =>
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

let mockFetch: ReturnType<typeof vi.fn>

function json(body: unknown) {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { 'content-type': 'application/json' },
  })
}
function empty() {
  return new Response(null, { status: 204, headers: { 'content-length': '0' } })
}
function lastCall() {
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

describe('players', () => {
  it('search encodes username', async () => {
    mockFetch.mockResolvedValue(json({ id: 'p1', username: 'alice' }))
    await players.search('ali ce')
    expect(lastCall().url).toContain('/players/search?username=ali%20ce')
  })
})

describe('friends', () => {
  it('list fetches GET', async () => {
    mockFetch.mockResolvedValue(json([]))
    await friends.list('p1')
    expect(lastCall().url).toContain('/players/p1/friends')
  })

  it('pending fetches GET', async () => {
    mockFetch.mockResolvedValue(json([]))
    await friends.pending('p1')
    expect(lastCall().url).toContain('/players/p1/friends/pending')
  })

  it('sendRequest sends POST', async () => {
    mockFetch.mockResolvedValue(json({}))
    await friends.sendRequest('p1', 'p2')
    const c = lastCall()
    expect(c.url).toContain('/players/p1/friends/p2')
    expect(c.method).toBe('POST')
  })

  it('acceptRequest sends PUT', async () => {
    mockFetch.mockResolvedValue(json({}))
    await friends.acceptRequest('p1', 'p2')
    const c = lastCall()
    expect(c.url).toContain('/players/p1/friends/p2/accept')
    expect(c.method).toBe('PUT')
  })

  it('declineRequest sends DELETE', async () => {
    mockFetch.mockResolvedValue(empty())
    await friends.declineRequest('p1', 'p2')
    const c = lastCall()
    expect(c.url).toContain('/players/p1/friends/p2/decline')
    expect(c.method).toBe('DELETE')
  })

  it('remove sends DELETE', async () => {
    mockFetch.mockResolvedValue(empty())
    await friends.remove('p1', 'p2')
    const c = lastCall()
    expect(c.url).toContain('/players/p1/friends/p2')
    expect(c.method).toBe('DELETE')
  })
})

describe('dmConversations', () => {
  it('list fetches GET', async () => {
    mockFetch.mockResolvedValue(json([]))
    await dmConversations.list('p1')
    expect(lastCall().url).toContain('/players/p1/dm/conversations')
  })
})

describe('blocks', () => {
  it('block sends POST', async () => {
    mockFetch.mockResolvedValue(json({}))
    await blocks.block('p1', 'p2')
    const c = lastCall()
    expect(c.url).toContain('/players/p1/block/p2')
    expect(c.method).toBe('POST')
  })

  it('unblock sends DELETE', async () => {
    mockFetch.mockResolvedValue(empty())
    await blocks.unblock('p1', 'p2')
    const c = lastCall()
    expect(c.url).toContain('/players/p1/block/p2')
    expect(c.method).toBe('DELETE')
  })
})

describe('presence', () => {
  it('check sends comma-separated ids', async () => {
    mockFetch.mockResolvedValue(json({ id1: true, id2: false }))
    await presence.check(['id1', 'id2'])
    expect(lastCall().url).toContain('/presence?ids=id1,id2')
  })
})
