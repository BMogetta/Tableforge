import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, fireEvent, act } from '@testing-library/react'
import { RateLimited } from '../RateLimited'

const mockNavigate = vi.fn()
vi.mock('@tanstack/react-router', () => ({
  useNavigate: () => mockNavigate,
}))

function renderPage() {
  act(() => {
    render(<RateLimited />)
  })
}

// The component chains setTimeout calls — each decrement schedules the next,
// so React must flush between ticks to trigger the subsequent setTimeout.
function advanceSeconds(n: number) {
  for (let i = 0; i < n; i++) {
    act(() => vi.advanceTimersByTime(1000))
  }
}

describe('RateLimited', () => {
  beforeEach(() => {
    vi.useFakeTimers()
    mockNavigate.mockReset()
  })

  afterEach(() => {
    vi.runAllTimers()
    vi.useRealTimers()
  })

  it('renders the initial countdown at 60', () => {
    renderPage()
    expect(screen.getByText('60')).toBeInTheDocument()
  })

  it('shows the "too many requests" heading', () => {
    renderPage()
    expect(screen.getByText('TOO MANY REQUESTS')).toBeInTheDocument()
  })

  it('decrements the countdown every second', () => {
    renderPage()
    expect(screen.getByText('60')).toBeInTheDocument()
    advanceSeconds(1)
    expect(screen.getByText('59')).toBeInTheDocument()
    advanceSeconds(1)
    expect(screen.getByText('58')).toBeInTheDocument()
  })

  it('shows singular "second" at 1', () => {
    renderPage()
    advanceSeconds(59)
    expect(screen.getByText('01')).toBeInTheDocument()
    expect(screen.getByText('second remaining')).toBeInTheDocument()
  })

  it('shows plural "seconds" above 1', () => {
    renderPage()
    expect(screen.getByText('seconds remaining')).toBeInTheDocument()
  })

  it('navigates to / when countdown reaches 0', () => {
    renderPage()
    advanceSeconds(60)
    expect(mockNavigate).toHaveBeenCalledWith({ to: '/' })
  })

  it('navigates to / immediately when Try Now is clicked', () => {
    renderPage()
    fireEvent.click(screen.getByText('Try Now'))
    expect(mockNavigate).toHaveBeenCalledWith({ to: '/' })
  })

  it('does not navigate before countdown ends', () => {
    renderPage()
    advanceSeconds(30)
    expect(mockNavigate).not.toHaveBeenCalled()
  })

  it('pads single-digit seconds with leading zero', () => {
    renderPage()
    advanceSeconds(51) // 60 - 51 = 9
    expect(screen.getByText('09')).toBeInTheDocument()
  })
})
