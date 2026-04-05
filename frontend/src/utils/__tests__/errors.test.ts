import { describe, it, expect } from 'vitest'
import { ok, error, toAppError, catchToAppError, type AppError } from '@/utils/errors'

// ---------------------------------------------------------------------------
// Result helpers
// ---------------------------------------------------------------------------

describe('ok()', () => {
  it('returns [null, value] tuple', () => {
    const [err, val] = ok(42)
    expect(err).toBeNull()
    expect(val).toBe(42)
  })
})

describe('error()', () => {
  it('returns [err, null] tuple', () => {
    const [err, val] = error({ reason: 'NOT_FOUND' as const, message: 'gone' })
    expect(val).toBeNull()
    expect(err).toEqual({ reason: 'NOT_FOUND', message: 'gone' })
  })
})

// ---------------------------------------------------------------------------
// toAppError — status code mapping
// ---------------------------------------------------------------------------

describe('toAppError', () => {
  // In test env (DEV=true), raw message is passed through.

  it('maps 400 to VALIDATION', () => {
    const e = toAppError(400, 'bad input')
    expect(e.reason).toBe('VALIDATION')
    expect(e.status).toBe(400)
    expect(e.message).toBe('bad input')
  })

  it('maps 401 to UNAUTHORIZED', () => {
    const e = toAppError(401, 'unauthorized')
    expect(e.reason).toBe('UNAUTHORIZED')
  })

  it('maps 403 to FORBIDDEN', () => {
    const e = toAppError(403, 'forbidden')
    expect(e.reason).toBe('FORBIDDEN')
  })

  it('maps 404 to NOT_FOUND', () => {
    const e = toAppError(404, 'not found')
    expect(e.reason).toBe('NOT_FOUND')
  })

  it('maps 409 to CONFLICT', () => {
    const e = toAppError(409, 'conflict')
    expect(e.reason).toBe('CONFLICT')
  })

  it('maps 429 to UNKNOWN (no special status)', () => {
    const e = toAppError(429, 'rate limited')
    expect(e.reason).toBe('UNKNOWN')
  })

  it('maps 500 to SERVER_ERROR', () => {
    const e = toAppError(500, 'internal')
    expect(e.reason).toBe('SERVER_ERROR')
  })

  it('maps 502 to SERVER_ERROR (any 5xx)', () => {
    const e = toAppError(502, 'bad gateway')
    expect(e.reason).toBe('SERVER_ERROR')
  })

  it('maps 503 to SERVER_ERROR', () => {
    const e = toAppError(503, 'unavailable')
    expect(e.reason).toBe('SERVER_ERROR')
  })

  it('maps undefined status to NETWORK_ERROR', () => {
    const e = toAppError(undefined, 'fetch failed')
    expect(e.reason).toBe('NETWORK_ERROR')
    expect(e.status).toBeUndefined()
  })

  it('maps unrecognized status (e.g. 418) to UNKNOWN', () => {
    const e = toAppError(418, "I'm a teapot")
    expect(e.reason).toBe('UNKNOWN')
  })

  it('exposes raw message in dev mode', () => {
    const e = toAppError(404, 'player xyz not found')
    // In test env (DEV=true), message is the raw server string
    expect(e.message).toBe('player xyz not found')
    // No code in dev mode
    expect(e.code).toBeUndefined()
  })
})

// ---------------------------------------------------------------------------
// catchToAppError — unknown error conversion
// ---------------------------------------------------------------------------

describe('catchToAppError', () => {
  it('converts an object with status + message (ApiError-like)', () => {
    const apiErr = { status: 403, message: 'forbidden' }
    const e = catchToAppError(apiErr)
    expect(e.reason).toBe('FORBIDDEN')
    expect(e.status).toBe(403)
    expect(e.message).toBe('forbidden')
  })

  it('converts a native Error to NETWORK_ERROR', () => {
    const e = catchToAppError(new TypeError('Failed to fetch'))
    expect(e.reason).toBe('NETWORK_ERROR')
    expect(e.status).toBeUndefined()
    expect(e.message).toBe('Failed to fetch')
  })

  it('converts a string to NETWORK_ERROR with generic message', () => {
    const e = catchToAppError('something broke')
    expect(e.reason).toBe('NETWORK_ERROR')
    expect(e.message).toBe('An unexpected error occurred.')
  })

  it('converts null to NETWORK_ERROR with generic message', () => {
    const e = catchToAppError(null)
    expect(e.reason).toBe('NETWORK_ERROR')
    expect(e.message).toBe('An unexpected error occurred.')
  })

  it('converts undefined to NETWORK_ERROR with generic message', () => {
    const e = catchToAppError(undefined)
    expect(e.reason).toBe('NETWORK_ERROR')
    expect(e.message).toBe('An unexpected error occurred.')
  })

  it('converts a number to NETWORK_ERROR with generic message', () => {
    const e = catchToAppError(42)
    expect(e.reason).toBe('NETWORK_ERROR')
    expect(e.message).toBe('An unexpected error occurred.')
  })

  it('converts an object with status but no message to NETWORK_ERROR', () => {
    // Missing 'message' — falls to instanceof Error check, then generic
    const e = catchToAppError({ status: 500 })
    expect(e.reason).toBe('NETWORK_ERROR')
    expect(e.message).toBe('An unexpected error occurred.')
  })

  it('handles object with both status and message for 500', () => {
    const e = catchToAppError({ status: 500, message: 'db down' })
    expect(e.reason).toBe('SERVER_ERROR')
    expect(e.status).toBe(500)
  })

  it('handles an Error subclass', () => {
    const e = catchToAppError(new RangeError('out of bounds'))
    expect(e.reason).toBe('NETWORK_ERROR')
    expect(e.message).toBe('out of bounds')
  })
})
