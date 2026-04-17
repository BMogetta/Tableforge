import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { CardPile } from '../CardPile'

describe('CardPile', () => {
  it('renders the card count', () => {
    render(<CardPile count={15} />)
    expect(screen.getByText('15')).toBeInTheDocument()
  })

  it('hides count when empty', () => {
    render(<CardPile count={0} />)
    expect(screen.queryByText('0')).not.toBeInTheDocument()
  })

  it('marks pile as empty via data attribute', () => {
    render(<CardPile count={0} />)
    expect(screen.getByTestId('card-pile')).toHaveAttribute('data-empty', 'true')
  })

  it('calls onDraw when clicked', () => {
    const onDraw = vi.fn()
    render(<CardPile count={5} onDraw={onDraw} />)
    const topCard = screen.getAllByTestId('card').pop()!
    fireEvent.click(topCard)
    expect(onDraw).toHaveBeenCalledOnce()
  })

  it('does not call onDraw when empty', () => {
    const onDraw = vi.fn()
    render(<CardPile count={0} onDraw={onDraw} />)
    // No card buttons should exist
    expect(screen.queryByTestId('card')).not.toBeInTheDocument()
  })

  it('renders top card content when provided', () => {
    render(
      <CardPile
        count={3}
        faceDown={false}
        topCard={{ name: 'Princess' }}
        renderCard={c => <span>{c.name}</span>}
      />,
    )
    expect(screen.getByText('Princess')).toBeInTheDocument()
  })

  it('caps visible layers at 3', () => {
    render(<CardPile count={50} />)
    expect(screen.getAllByTestId('card')).toHaveLength(3)
  })
})
