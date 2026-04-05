import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { z } from 'zod'

// Stub telemetry so it's a no-op — tracer.startActiveSpan calls the callback immediately.
vi.mock('../telemetry', () => ({
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

// Stub OpenTelemetry API propagation
vi.mock('@opentelemetry/api', () => ({
  SpanKind: { CLIENT: 1 },
  SpanStatusCode: { OK: 0, ERROR: 1 },
  context: { active: vi.fn() },
  propagation: { inject: vi.fn() },
}))

import { request, validatedRequest, ApiError, SchemaError } from '../api'

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

let mockFetch: ReturnType<typeof vi.fn>
let locationHref: string

beforeEach(() => {
  mockFetch = vi.fn()
  vi.stubGlobal('fetch', mockFetch)

  // Intercept window.location.href assignments
  locationHref = ''
  Object.defineProperty(window, 'location', {
    value: { href: '', protocol: 'https:', host: 'example.com' },
    writable: true,
    configurable: true,
  })
})

afterEach(() => {
  vi.restoreAllMocks()
  vi.unstubAllGlobals()
})

function jsonResponse(body: unknown, status = 200, headers?: Record<string, string>) {
  const h = new Headers({ 'content-type': 'application/json', ...headers })
  return new Response(JSON.stringify(body), { status, headers: h })
}

function emptyResponse(status: number) {
  return new Response(null, {
    status,
    headers: { 'content-length': '0' },
  })
}

// ---------------------------------------------------------------------------
// request()
// ---------------------------------------------------------------------------

describe('request()', () => {
  it('makes a GET request and returns JSON', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ name: 'Alice' }))

    const data = await request<{ name: string }>('/players/1')
    expect(data).toEqual({ name: 'Alice' })
    expect(mockFetch).toHaveBeenCalledWith(
      '/api/v1/players/1',
      expect.objectContaining({ credentials: 'include' }),
    )
  })

  it('makes a POST request with body', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ id: '123' }, 201))

    const data = await request<{ id: string }>('/rooms', {
      method: 'POST',
      body: JSON.stringify({ game: 'tictactoe' }),
    })
    expect(data).toEqual({ id: '123' })
  })

  it('returns undefined for 204 No Content', async () => {
    mockFetch.mockResolvedValueOnce(emptyResponse(204))

    const data = await request<void>('/rooms/1/leave', { method: 'POST' })
    expect(data).toBeUndefined()
  })

  it('returns undefined for content-length: 0', async () => {
    mockFetch.mockResolvedValueOnce(emptyResponse(200))

    const data = await request<void>('/ping')
    expect(data).toBeUndefined()
  })

  it('throws ApiError on 400', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ error: 'invalid input' }, 400))

    await expect(request('/rooms', { method: 'POST' })).rejects.toThrow(ApiError)
    try {
      await request('/rooms', { method: 'POST' })
    } catch (e) {
      // Need a fresh call since the first already threw
    }
  })

  it('throws ApiError with correct status and message', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ error: 'not found' }, 404))

    try {
      await request('/rooms/999')
      expect.unreachable('should have thrown')
    } catch (e) {
      expect(e).toBeInstanceOf(ApiError)
      expect((e as ApiError).status).toBe(404)
      expect((e as ApiError).message).toBe('not found')
    }
  })

  it('throws ApiError on 500', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ error: 'db down' }, 500))

    await expect(request('/health')).rejects.toThrow(ApiError)
  })

  it('redirects to /rate-limited on 429', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ error: 'too many requests' }, 429))

    await expect(request('/rooms')).rejects.toThrow(ApiError)
    expect(window.location.href).toBe('/rate-limited')
  })

  // --- 401 token refresh flow ---

  it('retries after successful token refresh on 401 token_expired', async () => {
    // First call: 401 with token_expired
    const expiredResponse = jsonResponse({ error: 'token_expired' }, 401)
    // Refresh call: success
    const refreshResponse = new Response(null, { status: 200 })
    // Retry call: success
    const successResponse = jsonResponse({ name: 'Alice' })

    mockFetch
      .mockResolvedValueOnce(expiredResponse) // initial request
      .mockResolvedValueOnce(refreshResponse) // POST /auth/refresh
      .mockResolvedValueOnce(successResponse) // retry

    const data = await request<{ name: string }>('/players/me')
    expect(data).toEqual({ name: 'Alice' })
    // 3 calls: original, refresh, retry
    expect(mockFetch).toHaveBeenCalledTimes(3)
    expect(mockFetch).toHaveBeenNthCalledWith(
      2,
      '/auth/refresh',
      expect.objectContaining({ method: 'POST' }),
    )
  })

  it('redirects to /login when refresh fails on 401 token_expired', async () => {
    const expiredResponse = jsonResponse({ error: 'token_expired' }, 401)
    const refreshFailResponse = new Response(null, { status: 401 })

    mockFetch
      .mockResolvedValueOnce(expiredResponse)
      .mockResolvedValueOnce(refreshFailResponse)

    await expect(request('/players/me')).rejects.toThrow(ApiError)
    expect(window.location.href).toBe('/login')
  })

  it('throws ApiError on 401 without token_expired (no refresh attempt)', async () => {
    // 401 with a different error — should NOT trigger refresh
    mockFetch.mockResolvedValueOnce(jsonResponse({ error: 'unauthorized' }, 401))

    await expect(request('/admin')).rejects.toThrow(ApiError)
    // Only 1 fetch call — no refresh attempt
    expect(mockFetch).toHaveBeenCalledTimes(1)
  })

  it('re-throws network errors (fetch rejection)', async () => {
    mockFetch.mockRejectedValueOnce(new TypeError('Failed to fetch'))

    await expect(request('/rooms')).rejects.toThrow('Failed to fetch')
  })

  it('falls back to statusText when error body has no error field', async () => {
    const res = new Response('{}', {
      status: 403,
      statusText: 'Forbidden',
      headers: { 'content-type': 'application/json' },
    })
    mockFetch.mockResolvedValueOnce(res)

    try {
      await request('/admin')
      expect.unreachable('should have thrown')
    } catch (e) {
      expect(e).toBeInstanceOf(ApiError)
      expect((e as ApiError).message).toBe('Forbidden')
    }
  })
})

// ---------------------------------------------------------------------------
// validatedRequest()
// ---------------------------------------------------------------------------

describe('validatedRequest()', () => {
  const playerSchema = z.object({
    id: z.string(),
    username: z.string(),
  })

  it('returns validated data on success', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ id: 'p1', username: 'Alice' }))

    const data = await validatedRequest(playerSchema, '/players/p1')
    expect(data).toEqual({ id: 'p1', username: 'Alice' })
  })

  it('strips extra fields (Zod default behavior)', async () => {
    mockFetch.mockResolvedValueOnce(
      jsonResponse({ id: 'p1', username: 'Alice', extra: true }),
    )

    const data = await validatedRequest(playerSchema, '/players/p1')
    // Zod strips extra fields by default
    expect(data).toEqual({ id: 'p1', username: 'Alice' })
  })

  it('throws SchemaError on invalid response data', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ id: 123, username: null }))

    await expect(validatedRequest(playerSchema, '/players/p1')).rejects.toThrow(SchemaError)
  })

  it('SchemaError includes details in dev mode', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ id: 123 }))

    try {
      await validatedRequest(playerSchema, '/players/p1')
      expect.unreachable('should have thrown')
    } catch (e) {
      expect(e).toBeInstanceOf(SchemaError)
      const se = e as SchemaError
      expect(se.details).toBeDefined()
      expect(se.details!.length).toBeGreaterThan(0)
      expect(se.message).toContain('/players/p1')
    }
  })

  it('propagates ApiError from failed request', async () => {
    mockFetch.mockResolvedValueOnce(jsonResponse({ error: 'not found' }, 404))

    await expect(validatedRequest(playerSchema, '/players/p1')).rejects.toThrow(ApiError)
  })
})

// ---------------------------------------------------------------------------
// ApiError / SchemaError classes
// ---------------------------------------------------------------------------

describe('ApiError', () => {
  it('has status and message properties', () => {
    const e = new ApiError(404, 'not found')
    expect(e.status).toBe(404)
    expect(e.message).toBe('not found')
    expect(e).toBeInstanceOf(Error)
  })
})

describe('SchemaError', () => {
  it('has name set to SchemaError', () => {
    const e = new SchemaError('validation failed')
    expect(e.name).toBe('SchemaError')
    expect(e).toBeInstanceOf(Error)
  })

  it('stores details array', () => {
    const e = new SchemaError('fail', ['id: Expected string', 'username: Required'])
    expect(e.details).toEqual(['id: Expected string', 'username: Required'])
  })

  it('details is undefined when not provided', () => {
    const e = new SchemaError('fail')
    expect(e.details).toBeUndefined()
  })
})
