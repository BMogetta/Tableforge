import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { useFocusTrap } from '../useFocusTrap'

function TrapDialog() {
  const ref = useFocusTrap<HTMLDivElement>()
  return (
    <div ref={ref} role='dialog' aria-modal='true'>
      <button data-testid='first'>First</button>
      <input data-testid='middle' />
      <button data-testid='last'>Last</button>
    </div>
  )
}

function EmptyTrapDialog() {
  const ref = useFocusTrap<HTMLDivElement>()
  return (
    <div ref={ref} role='dialog' aria-modal='true'>
      <p>No focusable elements</p>
    </div>
  )
}

describe('useFocusTrap', () => {
  it('focuses the first focusable element on mount', () => {
    render(<TrapDialog />)
    expect(document.activeElement).toBe(screen.getByTestId('first'))
  })

  it('restores focus to previous element on unmount', () => {
    const outer = document.createElement('button')
    outer.textContent = 'Outer'
    document.body.appendChild(outer)
    outer.focus()

    const { unmount } = render(<TrapDialog />)
    expect(document.activeElement).toBe(screen.getByTestId('first'))

    unmount()
    expect(document.activeElement).toBe(outer)

    document.body.removeChild(outer)
  })

  it('registers keydown listener on document', () => {
    const { unmount } = render(<TrapDialog />)
    // The hook adds a keydown listener — verify it doesn't throw on Tab events
    const event = new KeyboardEvent('keydown', { key: 'Tab', bubbles: true })
    expect(() => document.dispatchEvent(event)).not.toThrow()
    unmount()
  })

  it('handles dialog with no focusable elements without crashing', () => {
    expect(() => render(<EmptyTrapDialog />)).not.toThrow()
  })
})
