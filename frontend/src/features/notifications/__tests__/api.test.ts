import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import { notifications } from '../api'

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

describe('notifications', () => {
  it('list fetches with params', async () => {
    mockFetch.mockResolvedValue(json({ items: [], total: 0 }))
    await notifications.list('p1', true, 10, 5)
    const url = lastCall().url
    expect(url).toContain('/players/p1/notifications')
    expect(url).toContain('include_read=true')
    expect(url).toContain('limit=10')
    expect(url).toContain('offset=5')
  })

  it('markRead sends POST', async () => {
    mockFetch.mockResolvedValue(empty())
    await notifications.markRead('p1', 'n1')
    const c = lastCall()
    expect(c.url).toContain('/notifications/n1/read')
    expect(c.method).toBe('POST')
  })

  it('accept sends POST', async () => {
    mockFetch.mockResolvedValue(empty())
    await notifications.accept('p1', 'n1')
    const c = lastCall()
    expect(c.url).toContain('/notifications/n1/accept')
    expect(c.method).toBe('POST')
  })

  it('decline sends POST', async () => {
    mockFetch.mockResolvedValue(empty())
    await notifications.decline('p1', 'n1')
    const c = lastCall()
    expect(c.url).toContain('/notifications/n1/decline')
    expect(c.method).toBe('POST')
  })
})
