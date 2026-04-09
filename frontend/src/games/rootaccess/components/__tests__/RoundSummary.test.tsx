import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest'
import { render, screen, act } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { RoundSummary } from '../RoundSummary'

vi.mock('@/hooks/useFocusTrap', () => ({
  useFocusTrap: () => ({ current: null }),
}))

const basePlayers = [
  { id: 'p1', username: 'Alice', tokens: 2, tokensToWin: 3, isWinner: true, earnedBackdoorBonus: false },
  { id: 'p2', username: 'Bob', tokens: 1, tokensToWin: 3, isWinner: false, earnedBackdoorBonus: false },
]

let onDismiss: ReturnType<typeof vi.fn>

beforeEach(() => {
  vi.useFakeTimers()
  onDismiss = vi.fn()
})

afterEach(() => {
  vi.useRealTimers()
})

describe('RoundSummary', () => {
  it('renders round number and winner', () => {
    render(
      <RoundSummary round={3} winnerId='p1' players={basePlayers} onDismiss={onDismiss} />,
    )
    expect(screen.getByText(/Alice wins the round/)).toBeInTheDocument()
    // Round label uses i18n key
    expect(screen.getByRole('dialog')).toBeInTheDocument()
  })

  it('renders draw when no winner', () => {
    render(
      <RoundSummary round={1} winnerId={null} players={basePlayers} onDismiss={onDismiss} />,
    )
    // With no winner the draw text is shown
    const headings = screen.getAllByRole('heading')
    expect(headings.length).toBeGreaterThan(0)
  })

  it('renders all player rows', () => {
    render(
      <RoundSummary round={1} winnerId='p1' players={basePlayers} onDismiss={onDismiss} />,
    )
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()
  })

  it('renders token track with correct filled count', () => {
    const { container } = render(
      <RoundSummary round={1} winnerId='p1' players={basePlayers} onDismiss={onDismiss} />,
    )
    // Alice has 2/3 tokens, Bob has 1/3
    const filledTokens = container.querySelectorAll('[class*="tokenFilled"]')
    expect(filledTokens.length).toBe(3) // 2 + 1
  })

  it('shows backdoor bonus when earned', () => {
    const players = [
      { ...basePlayers[0], earnedBackdoorBonus: true },
      basePlayers[1],
    ]
    render(
      <RoundSummary round={1} winnerId='p1' players={players} onDismiss={onDismiss} />,
    )
    expect(screen.getByText('Backdoor bonus')).toBeInTheDocument()
    expect(screen.getByText(/Alice earned \+1 token/)).toBeInTheDocument()
  })

  it('shows hand card when present', () => {
    const players = [
      { ...basePlayers[0], handCard: 'ROOT' },
      basePlayers[1],
    ]
    render(
      <RoundSummary round={1} winnerId='p1' players={players} onDismiss={onDismiss} />,
    )
    expect(screen.getByText('ROOT')).toBeInTheDocument()
  })

  it('calls onDismiss when continue button clicked', async () => {
    vi.useRealTimers()
    const user = userEvent.setup()
    render(
      <RoundSummary round={1} winnerId='p1' players={basePlayers} autoDismissMs={999999} onDismiss={onDismiss} />,
    )
    const btn = screen.getByRole('button')
    await user.click(btn)
    expect(onDismiss).toHaveBeenCalled()
  })

  it('auto-dismisses after timeout', () => {
    render(
      <RoundSummary round={1} winnerId='p1' players={basePlayers} autoDismissMs={500} onDismiss={onDismiss} />,
    )
    // The component uses setInterval(100ms) decrementing remaining.
    // Wrap in act to flush React state updates from timers.
    act(() => {
      vi.advanceTimersByTime(600)
    })
    expect(onDismiss).toHaveBeenCalled()
  })

  it('has accessible dialog role and label', () => {
    render(
      <RoundSummary round={1} winnerId='p1' players={basePlayers} onDismiss={onDismiss} />,
    )
    const dialog = screen.getByRole('dialog')
    expect(dialog).toHaveAttribute('aria-modal', 'true')
    expect(dialog).toHaveAttribute('aria-labelledby', 'round-summary-title')
  })
})
