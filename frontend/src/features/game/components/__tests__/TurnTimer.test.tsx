import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import { TurnTimer } from '../TurnTimer'

describe('TurnTimer', () => {
  beforeEach(() => {
    vi.useFakeTimers()
  })

  afterEach(() => {
    vi.useRealTimers()
  })

  function nowISO() {
    return new Date().toISOString()
  }

  it('shows remaining seconds', () => {
    render(<TurnTimer turnTimeoutSecs={30} lastMoveAt={nowISO()} isOver={false} isSuspended={false} />)
    expect(screen.getByTestId('turn-timer')).toHaveTextContent('30s')
  })

  it('counts down every second', () => {
    render(<TurnTimer turnTimeoutSecs={30} lastMoveAt={nowISO()} isOver={false} isSuspended={false} />)
    act(() => { vi.advanceTimersByTime(3_000) })
    expect(screen.getByTestId('turn-timer')).toHaveTextContent('27s')
  })

  it('does not go below 0', () => {
    render(<TurnTimer turnTimeoutSecs={2} lastMoveAt={nowISO()} isOver={false} isSuspended={false} />)
    act(() => { vi.advanceTimersByTime(5_000) })
    expect(screen.getByTestId('turn-timer')).toHaveTextContent('0s')
  })

  it('hides when game is over', () => {
    render(<TurnTimer turnTimeoutSecs={30} lastMoveAt={nowISO()} isOver={true} isSuspended={false} />)
    expect(screen.queryByTestId('turn-timer')).not.toBeInTheDocument()
  })

  it('hides when timeout is 0', () => {
    render(<TurnTimer turnTimeoutSecs={0} lastMoveAt={nowISO()} isOver={false} isSuspended={false} />)
    expect(screen.queryByTestId('turn-timer')).not.toBeInTheDocument()
  })

  it('freezes when suspended and does not tick', () => {
    render(<TurnTimer turnTimeoutSecs={30} lastMoveAt={nowISO()} isOver={false} isSuspended={true} />)
    const initial = screen.getByTestId('turn-timer').textContent

    act(() => { vi.advanceTimersByTime(5_000) })

    // Value must not have changed
    expect(screen.getByTestId('turn-timer')).toHaveTextContent(initial!)
  })

  it('shows (paused) label when suspended', () => {
    render(<TurnTimer turnTimeoutSecs={30} lastMoveAt={nowISO()} isOver={false} isSuspended={true} />)
    expect(screen.getByTestId('turn-timer')).toHaveTextContent('(paused)')
  })

  it('after resume shows 40% penalty time (backend sets lastMoveAt accordingly)', () => {
    const startTime = nowISO()

    const { rerender } = render(
      <TurnTimer turnTimeoutSecs={30} lastMoveAt={startTime} isOver={false} isSuspended={false} />,
    )

    // Tick 10 seconds → 20s remaining
    act(() => { vi.advanceTimersByTime(10_000) })
    expect(screen.getByTestId('turn-timer')).toHaveTextContent('20s')

    // Pause
    rerender(
      <TurnTimer turnTimeoutSecs={30} lastMoveAt={startTime} isOver={false} isSuspended={true} />,
    )
    expect(screen.getByTestId('turn-timer')).toHaveTextContent('20s')

    // 15 seconds pass while paused — value must NOT change
    act(() => { vi.advanceTimersByTime(15_000) })
    expect(screen.getByTestId('turn-timer')).toHaveTextContent('20s')

    // Resume — backend sets lastMoveAt = NOW() - 60% * 30 = NOW() - 18s
    // so remaining = 30 - 18 = 12s (40% penalty)
    const resumeLastMoveAt = new Date(Date.now() - 18_000).toISOString()
    rerender(
      <TurnTimer turnTimeoutSecs={30} lastMoveAt={resumeLastMoveAt} isOver={false} isSuspended={false} />,
    )
    expect(screen.getByTestId('turn-timer')).toHaveTextContent('12s')

    // Ticks normally from there
    act(() => { vi.advanceTimersByTime(5_000) })
    expect(screen.getByTestId('turn-timer')).toHaveTextContent('7s')
  })

  it('shows urgent style in last 10 seconds', () => {
    render(<TurnTimer turnTimeoutSecs={12} lastMoveAt={nowISO()} isOver={false} isSuspended={false} />)
    act(() => { vi.advanceTimersByTime(3_000) })
    // 9s remaining — should have urgent class
    const el = screen.getByTestId('turn-timer')
    expect(el.className).toContain('Urgent')
  })

  it('does not show urgent style when paused even if time is low', () => {
    render(<TurnTimer turnTimeoutSecs={5} lastMoveAt={nowISO()} isOver={false} isSuspended={true} />)
    const el = screen.getByTestId('turn-timer')
    expect(el.className).not.toContain('Urgent')
  })
})
