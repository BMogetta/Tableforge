import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { CardDisplay, CARD_META } from '../CardDisplay'

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

  it('sets data-facedown when faceDown prop is true', () => {
    render(<CardDisplay card='ping' faceDown />)
    const card = screen.getByTestId('card-display')
    expect(card).toHaveAttribute('data-facedown', 'true')
  })

  it('marks card as selected via data attribute', () => {
    render(<CardDisplay card='ping' selected />)
    expect(screen.getByTestId('card-display')).toHaveAttribute('data-selected', 'true')
  })

  it('marks card as disabled via data attribute', () => {
    render(<CardDisplay card='ping' disabled />)
    expect(screen.getByTestId('card-display')).toHaveAttribute('data-disabled', 'true')
  })

  it('calls onClick when clicked and not disabled or faceDown', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(<CardDisplay card='ping' onClick={onClick} />)
    await user.click(screen.getByTestId('card'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('does not call onClick when disabled', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(<CardDisplay card='ping' onClick={onClick} disabled />)
    await user.click(screen.getByTestId('card'))
    expect(onClick).not.toHaveBeenCalled()
  })

  it('renders aria-label indicating face-down state', () => {
    render(<CardDisplay card='ping' faceDown />)
    expect(screen.getByTestId('card')).toHaveAttribute('aria-label', 'Face-down card')
  })

  it('calls onClick on Enter key press', async () => {
    const user = userEvent.setup()
    const onClick = vi.fn()
    render(<CardDisplay card='ping' onClick={onClick} />)
    screen.getByTestId('card').focus()
    await user.keyboard('{Enter}')
    expect(onClick).toHaveBeenCalledOnce()
  })
})
