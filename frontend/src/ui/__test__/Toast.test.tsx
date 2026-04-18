import { act, fireEvent, render, screen } from '@testing-library/react'
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest'
import type { AppError } from '@/utils/errors'
import { ToastProvider, useToast } from '../Toast'

// ---------------------------------------------------------------------------
// Test harness
// ---------------------------------------------------------------------------

function TestHarness() {
  const toast = useToast()

  const devError: AppError = {
    reason: 'SERVER_ERROR',
    status: 500,
    message: 'internal server error',
  }

  const prodError: AppError = {
    reason: 'SERVER_ERROR',
    status: 500,
    message: 'A server error occurred. Please try again.',
    code: 'ERR_SRVE',
  }

  return (
    <div>
      <button type='button' data-testid='show-error' onClick={() => toast.showError(devError)}>
        Error
      </button>
      <button
        type='button'
        data-testid='show-prod-error'
        onClick={() => toast.showError(prodError)}
      >
        Prod Error
      </button>
      <button
        type='button'
        data-testid='show-warning'
        onClick={() => toast.showWarning('watch out')}
      >
        Warning
      </button>
      <button type='button' data-testid='show-info' onClick={() => toast.showInfo('heads up')}>
        Info
      </button>
    </div>
  )
}

function renderToast() {
  return render(
    <ToastProvider>
      <TestHarness />
    </ToastProvider>,
  )
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

describe('ToastProvider / useToast', () => {
  beforeEach(() => vi.useFakeTimers())
  afterEach(() => {
    vi.runAllTimers()
    vi.useRealTimers()
  })

  it('throws when used outside provider', () => {
    const spy = vi.spyOn(console, 'error').mockImplementation(() => {})
    expect(() => render(<TestHarness />)).toThrow('useToast must be used inside ToastProvider')
    spy.mockRestore()
  })

  it('shows an error toast', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-error'))
    expect(screen.getByText('internal server error')).toBeInTheDocument()
  })

  it('shows a warning toast', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-warning'))
    expect(screen.getByText('watch out')).toBeInTheDocument()
  })

  it('shows an info toast', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-info'))
    expect(screen.getByText('heads up')).toBeInTheDocument()
  })

  it('shows error code when present', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-prod-error'))
    expect(screen.getByText('ERR_SRVE')).toBeInTheDocument()
  })

  it('auto-dismisses after 4 seconds', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-error'))
    expect(screen.getByText('internal server error')).toBeInTheDocument()

    act(() => {
      vi.advanceTimersByTime(4000)
    })

    expect(screen.queryByText('internal server error')).toBeNull()
  })

  it('remains visible before 4 seconds have elapsed', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-error'))

    act(() => {
      vi.advanceTimersByTime(3999)
    })

    expect(screen.getByText('internal server error')).toBeInTheDocument()
  })

  it('dismisses when clicking the dismiss button', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-error'))
    fireEvent.click(screen.getByRole('button', { name: 'Dismiss notification' }))
    expect(screen.queryByText('internal server error')).toBeNull()
  })

  it('caps at 3 visible toasts', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-warning'))
    fireEvent.click(screen.getByTestId('show-warning'))
    fireEvent.click(screen.getByTestId('show-warning'))
    fireEvent.click(screen.getByTestId('show-warning')) // 4th — evicts oldest

    expect(screen.getAllByRole('alert')).toHaveLength(3)
  })

  it('multiple toasts are all visible before any timer fires', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-error'))
    fireEvent.click(screen.getByTestId('show-warning'))

    expect(screen.getByText('internal server error')).toBeInTheDocument()
    expect(screen.getByText('watch out')).toBeInTheDocument()
  })

  it('each toast has an independent timer — dismissing one does not affect others', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-error'))

    // Advance 2 seconds, then add a second toast.
    act(() => {
      vi.advanceTimersByTime(2000)
    })
    fireEvent.click(screen.getByTestId('show-warning'))

    // Advance 2 more seconds — first toast (4s total) should auto-dismiss.
    act(() => {
      vi.advanceTimersByTime(2000)
    })

    expect(screen.queryByText('internal server error')).toBeNull()
    // Second toast added 2s ago — still has 2s left.
    expect(screen.getByText('watch out')).toBeInTheDocument()
  })

  it('manually dismissed toast does not affect sibling timers', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-error'))
    fireEvent.click(screen.getByTestId('show-warning'))

    // Click to dismiss the first alert.
    const alerts = screen.getAllByRole('alert')
    fireEvent.click(alerts[0])

    // Warning toast should still be visible.
    expect(screen.getByText('watch out')).toBeInTheDocument()
  })

  it('does not render container when no toasts are present', () => {
    renderToast()
    expect(screen.queryByRole('alert')).toBeNull()
  })

  it('container disappears after all toasts are dismissed', () => {
    renderToast()
    fireEvent.click(screen.getByTestId('show-error'))
    act(() => {
      vi.advanceTimersByTime(4000)
    })
    expect(screen.queryByRole('alert')).toBeNull()
  })
})
