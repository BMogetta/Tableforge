/**
 * Represents the outcome of an operation that can either succeed or fail.
 *
 * Tuple shape:
 *   - `[ErrorObject, null]` on failure
 *   - `[null, SuccessValue]` on success
 *
 * The `reason` discriminant on error objects enables exhaustive switch handling
 * enforced by TypeScript (`err satisfies never` in the default branch).
 *
 * @template S - The success value type.
 * @template E - The error object type. Must have a `reason` string discriminant.
 */
import { isDev } from '@/lib/env'

export type Result<S, E extends { reason: string }> = [E, null] | [null, S]

/**
 * Wraps a success value into a Result.
 */
export function ok<S>(result: S): Result<S, never> {
  return [null, result]
}

/**
 * Wraps an error object into a Result.
 * The `reason` field acts as a discriminant for exhaustive switch handling.
 */
export function error<const R extends string, E extends { reason: R }>(err: E): Result<never, E> {
  return [err, null]
}

// ---------------------------------------------------------------------------
// API error types
// ---------------------------------------------------------------------------

/**
 * Typed reasons for API errors.
 * Add new reasons here as needed — TypeScript will enforce exhaustive handling
 * in every switch that covers AppError.
 */
export type ApiErrorReason =
  | 'UNAUTHORIZED' // 401
  | 'FORBIDDEN' // 403
  | 'NOT_FOUND' // 404
  | 'CONFLICT' // 409
  | 'VALIDATION' // 400
  | 'SERVER_ERROR' // 5xx
  | 'NETWORK_ERROR' // fetch failed (no response)
  | 'UNKNOWN' // fallback

/**
 * Structured application error.
 *
 * In development: `message` contains the raw server error string.
 * In production:  `message` is a generic friendly string; `code` is a short
 *                 deterministic identifier the user can report (e.g. "ERR_A3F2").
 */
export interface AppError {
  reason: ApiErrorReason
  status?: number
  /** Human-readable message. Raw in dev, generic in prod. */
  message: string
  /** Short deterministic code shown in prod for support lookup. */
  code?: string
}

// ---------------------------------------------------------------------------
// Conversion helpers
// ---------------------------------------------------------------------------

/**
 * Maps an HTTP status code to a typed ApiErrorReason.
 */
function statusToReason(status?: number): ApiErrorReason {
  if (!status) return 'NETWORK_ERROR'
  if (status === 400) return 'VALIDATION'
  if (status === 401) return 'UNAUTHORIZED'
  if (status === 403) return 'FORBIDDEN'
  if (status === 404) return 'NOT_FOUND'
  if (status === 409) return 'CONFLICT'
  if (status >= 500) return 'SERVER_ERROR'
  return 'UNKNOWN'
}

/**
 * Returns a friendly message for production display.
 */
function friendlyMessage(reason: ApiErrorReason): string {
  switch (reason) {
    case 'UNAUTHORIZED':
      return 'You need to log in to do that.'
    case 'FORBIDDEN':
      return "You don't have permission to do that."
    case 'NOT_FOUND':
      return 'The requested resource was not found.'
    case 'CONFLICT':
      return 'This action conflicts with the current state.'
    case 'VALIDATION':
      return 'The request was invalid. Check your input.'
    case 'SERVER_ERROR':
      return 'A server error occurred. Please try again.'
    case 'NETWORK_ERROR':
      return 'Could not reach the server. Check your connection.'
    case 'UNKNOWN':
      return 'Something went wrong.'
    default:
      reason satisfies never
      return 'Something went wrong.'
  }
}

/**
 * Generates a short deterministic error code for production support lookup.
 * Same reason + status always produces the same code.
 * Example: "ERR_A3F2"
 */
function generateCode(reason: ApiErrorReason, status?: number): string {
  const raw = reason + (status ?? 0)
  // btoa is available in all modern browsers and Node 16+
  const encoded = btoa(raw)
    .replace(/[^A-Z0-9]/gi, '')
    .toUpperCase()
    .slice(0, 4)
  return `ERR_${encoded}`
}

/**
 * Converts a raw API error (status + message from the server) into a typed
 * AppError. Behaviour differs by environment:
 *
 * - Dev:  exposes the raw server message for fast debugging
 * - Prod: shows a friendly message + a reportable code
 */
export function toAppError(status: number | undefined, rawMessage: string): AppError {
  const reason = statusToReason(status)
  if (isDev) {
    return { reason, status, message: rawMessage }
  }
  return {
    reason,
    status,
    message: friendlyMessage(reason),
    code: generateCode(reason, status),
  }
}

/**
 * Wraps an unknown caught value into an AppError.
 * Use this in catch blocks when you don't know the error type.
 *
 * @example
 * try { ... } catch (e) { return error(catchToAppError(e)) }
 */
export function catchToAppError(e: unknown): AppError {
  if (e && typeof e === 'object' && 'status' in e && 'message' in e) {
    // Looks like an ApiError from api.ts
    return toAppError((e as { status: number }).status, (e as { message: string }).message)
  }
  if (e instanceof Error) {
    return toAppError(undefined, e.message)
  }
  return toAppError(undefined, 'An unexpected error occurred.')
}
