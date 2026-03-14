import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import HandDisplay from '../HandDisplay'
import type { CardName } from '../CardDisplay'

describe('HandDisplay', () => {
  it('renders all cards in hand', () => {
    render(
      <HandDisplay
        cards={['guard', 'priest'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByText('Guard')).toBeInTheDocument()
    expect(screen.getByText('Priest')).toBeInTheDocument()
  })

  it('shows empty message when hand is empty', () => {
    render(
      <HandDisplay
        cards={[]}
        selectedCard={null}
        disabled={false}
        onSelect={vi.fn()}
      />
    )
    expect(screen.getByText('No cards in hand')).toBeInTheDocument()
  })

  it('calls onSelect with the clicked card', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(
      <HandDisplay
        cards={['guard', 'spy'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={onSelect}
      />
    )
    await user.click(screen.getAllByRole('button')[0])
    expect(onSelect).toHaveBeenCalledWith('guard')
  })

  it('does not call onSelect when disabled', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(
      <HandDisplay
        cards={['guard', 'spy'] as CardName[]}
        selectedCard={null}
        disabled={true}
        onSelect={onSelect}
      />
    )
    expect(screen.queryByRole('button')).not.toBeInTheDocument()
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('marks the selected card via data-selected attribute', () => {
    render(
      <HandDisplay
        cards={['guard', 'spy'] as CardName[]}
        selectedCard={'guard'}
        disabled={false}
        onSelect={vi.fn()}
      />
    )
    const cards = screen.getAllByTestId('card-display')
    expect(cards[0]).toHaveAttribute('data-selected', 'true')
    expect(cards[1]).toHaveAttribute('data-selected', 'false')
  })

  it('shows Countess must play label for blocked cards', () => {
    render(
      <HandDisplay
        cards={['king', 'countess'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={vi.fn()}
        blockedCards={['king']}
      />
    )
    expect(screen.getByText('Must play Countess')).toBeInTheDocument()
  })

  it('does not call onSelect for blocked cards', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(
      <HandDisplay
        cards={['king', 'countess'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={onSelect}
        blockedCards={['king']}
      />
    )
    // Only countess should be clickable — king is blocked.
    const buttons = screen.getAllByRole('button')
    expect(buttons).toHaveLength(1)
    await user.click(buttons[0])
    expect(onSelect).toHaveBeenCalledWith('countess')
  })
})