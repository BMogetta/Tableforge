import { useCallback, useState } from 'react'
import { error, ok, type Result } from '@/utils/errors'

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type ClipboardErrorReason = 'CLIPBOARD_UNAVAILABLE'

export interface ClipboardError {
  reason: ClipboardErrorReason
  message: string
}

export type ClipboardResult = Result<true, ClipboardError>

// ---------------------------------------------------------------------------
// Hook
// ---------------------------------------------------------------------------

/**
 * Wraps the Clipboard API with the Result pattern.
 *
 * Returns:
 * - `copy(text)` — writes text to the clipboard. Returns `[null, true]` on
 *   success or `[ClipboardError, null]` on failure. Never throws.
 * - `copied` — true for `resetMs` milliseconds after a successful copy.
 *   Use this to show transient "Copied!" feedback in the UI.
 *
 * The hook does NOT show any toast or feedback — the caller decides how to
 * handle errors (typically via toast.showError).
 *
 * @param resetMs - How long `copied` stays true after a successful copy.
 *                  Defaults to 2000ms.
 *
 * @example
 * const { copy, copied } = useClipboard()
 * const [err] = await copy(room.code)
 * if (err) toast.showError({ reason: 'UNKNOWN', message: err.message })
 *
 * @testability
 * Mock `navigator.clipboard.writeText` to resolve or reject.
 * Assert that `copied` toggles correctly and the Result shape is correct.
 */
export function useClipboard(resetMs = 2000) {
  const [copied, setCopied] = useState(false)

  const copy = useCallback(
    async (text: string): Promise<ClipboardResult> => {
      try {
        await navigator.clipboard.writeText(text)
        setCopied(true)
        setTimeout(() => setCopied(false), resetMs)
        return ok(true)
      } catch {
        return error({
          reason: 'CLIPBOARD_UNAVAILABLE',
          message: 'Failed to copy to clipboard',
        })
      }
    },
    [resetMs],
  )

  return { copy, copied }
}
