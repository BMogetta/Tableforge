import { type RefObject, useEffect, useRef } from 'react'

const FOCUSABLE = [
  'a[href]',
  'button:not(:disabled)',
  'input:not(:disabled)',
  'select:not(:disabled)',
  'textarea:not(:disabled)',
  '[tabindex]:not([tabindex="-1"])',
].join(', ')

/**
 * Traps keyboard focus inside a container element.
 *
 * When the dialog mounts, focus moves to the first focusable element.
 * Tab / Shift+Tab cycle within the container. On unmount, focus returns
 * to the element that was focused before the trap activated.
 *
 * Usage:
 *   const ref = useFocusTrap<HTMLDivElement>()
 *   <div ref={ref} role="dialog" aria-modal="true">...</div>
 */
export function useFocusTrap<T extends HTMLElement>(): RefObject<T | null> {
  const containerRef = useRef<T | null>(null)
  const previousFocusRef = useRef<HTMLElement | null>(null)

  useEffect(() => {
    const container = containerRef.current
    if (!container) return

    // Save the currently focused element to restore later.
    previousFocusRef.current = document.activeElement as HTMLElement

    // Focus the first focusable element inside the container.
    const focusables = container.querySelectorAll<HTMLElement>(FOCUSABLE)
    if (focusables.length > 0) {
      focusables[0].focus()
    }

    function handleKeyDown(e: KeyboardEvent) {
      if (e.key !== 'Tab') return

      const els = container!.querySelectorAll<HTMLElement>(FOCUSABLE)
      if (els.length === 0) return

      const first = els[0]
      const last = els[els.length - 1]

      if (e.shiftKey) {
        if (document.activeElement === first) {
          e.preventDefault()
          last.focus()
        }
      } else if (document.activeElement === last) {
        e.preventDefault()
        first.focus()
      }
    }

    document.addEventListener('keydown', handleKeyDown)

    return () => {
      document.removeEventListener('keydown', handleKeyDown)
      // Restore focus to the element that was focused before the trap.
      previousFocusRef.current?.focus()
    }
  }, [])

  return containerRef
}
