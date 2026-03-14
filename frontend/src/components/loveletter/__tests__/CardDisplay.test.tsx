import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import CardDisplay, { CARD_META } from '../CardDisplay'

describe('CardDisplay', () => {
  it('renders card name and effect for each card', () => {
    const cards = Object.keys(CARD_META) as Array<keyof typeof CARD_META>
    for (const card of cards) {
      const { unmount } = render(<CardDisplay card={card} />)
      const meta = CARD_META[card]
      expect(screen.getByText(meta.label)).toBeInTheDocument()
      expect(screen.getByText(meta.effect)).toBeInTheDocument()
      unmount()
    }
  })

  it('renders face-down when faceDown prop is true', () => {
    render(<CardDisplay card="guard" faceDown />)
    const card = screen.getByTestId('card-display')
    expect(card).toHaveAttribute('data-facedown', 'true')
    expect(screen.queryByText('Guard')).not.toBeInTheDocument()
  })

  it('marks card as selected via data attribute', () => {
    render(<CardDisplay card="guard" selected />)
    expect(screen.getByTestId('card-display')).toHaveAttribute('data-selected', 'true')
  })

  it('marks card as disabled via data attribute', () => {
    render(<CardDisplay card="guard" disabled />)
    expect(screen.getByTestId('card-display')).toHaveAttribute('data-disabled', 'true')
  })

  it('calls onClick when clicked and not disabled or faceDown', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(<CardDisplay card="guard" onClick={onClick} />)
    await user.click(screen.getByRole('button'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('does not render as button when disabled', () => {
    render(<CardDisplay card="guard" onClick={vi.fn()} disabled />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('does not render as button when faceDown', () => {
    render(<CardDisplay card="guard" onClick={vi.fn()} faceDown />)
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
  })

  it('calls onClick on Enter key press', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(<CardDisplay card="guard" onClick={onClick} />)
    screen.getByRole('button').focus()
    await user.keyboard('{Enter}')
    expect(onClick).toHaveBeenCalledOnce()
  })
})