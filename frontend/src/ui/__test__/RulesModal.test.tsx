import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import { useSettingsStore } from '@/stores/settingsStore'
import { RulesModal } from '../RulesModal'

// Mock focus trap to avoid ref issues in test
vi.mock('@/hooks/useFocusTrap', () => ({
  useFocusTrap: () => ({ current: null }),
}))

describe('RulesModal', () => {
  beforeEach(() => {
    useSettingsStore.setState({ settings: { show_move_hints: true } as any })
  })

  it('renders with game tabs', () => {
    render(<RulesModal onClose={vi.fn()} />)
    expect(screen.getByText('Game Rules')).toBeInTheDocument()
    expect(screen.getByText('TicTacToe')).toBeInTheDocument()
    expect(screen.getByText('Root Access')).toBeInTheDocument()
  })

  it('defaults to first tab (TicTacToe)', () => {
    render(<RulesModal onClose={vi.fn()} />)
    expect(screen.getByText('Objective')).toBeInTheDocument()
    expect(screen.getByText(/three of your marks/i)).toBeInTheDocument()
  })

  it('pre-selects tab when initialGameId is provided', () => {
    render(<RulesModal initialGameId='rootaccess' onClose={vi.fn()} />)
    expect(screen.getByText('Card Reference')).toBeInTheDocument()
    expect(screen.getByText('PING')).toBeInTheDocument()
  })

  it('switches tabs on click', async () => {
    const user = userEvent.setup()
    render(<RulesModal onClose={vi.fn()} />)
    await user.click(screen.getByText('Root Access'))
    expect(screen.getByText('Card Reference')).toBeInTheDocument()
  })

  it('calls onClose when close button is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<RulesModal onClose={onClose} />)
    await user.click(screen.getByLabelText('Close'))
    expect(onClose).toHaveBeenCalledOnce()
  })

  it('calls onClose when backdrop is clicked', async () => {
    const user = userEvent.setup()
    const onClose = vi.fn()
    render(<RulesModal onClose={onClose} />)
    await user.click(screen.getByLabelText('Close dialog'))
    expect(onClose).toHaveBeenCalledOnce()
  })
})
