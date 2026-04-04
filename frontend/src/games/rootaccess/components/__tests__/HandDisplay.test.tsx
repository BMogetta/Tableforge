import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { HandDisplay } from '../HandDisplay'
import type { CardName } from '../CardDisplay'

describe('HandDisplay', () => {
  it('renders all cards in hand', () => {
    render(
      <HandDisplay
        cards={['ping', 'sniffer'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={vi.fn()}
      />,
    )
    expect(screen.getByText('PING')).toBeInTheDocument()
    expect(screen.getByText('SNIFFER')).toBeInTheDocument()
  })

  it('shows empty message when hand is empty', () => {
    render(<HandDisplay cards={[]} selectedCard={null} disabled={false} onSelect={vi.fn()} />)
    expect(screen.getByText('No cards in hand')).toBeInTheDocument()
  })

  it('calls onSelect with the clicked card', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(
      <HandDisplay
        cards={['ping', 'backdoor'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={onSelect}
      />,
    )
    const cards = screen.getAllByTestId('card')
    await user.click(cards[0])
    expect(onSelect).toHaveBeenCalledWith('ping')
  })

  it('does not call onSelect when disabled', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(
      <HandDisplay
        cards={['ping', 'backdoor'] as CardName[]}
        selectedCard={null}
        disabled={true}
        onSelect={onSelect}
      />,
    )
    // Cards are still rendered but disabled — clicking should not fire
    const cards = screen.getAllByTestId('card')
    await user.click(cards[0])
    expect(onSelect).not.toHaveBeenCalled()
  })

  it('shows ENCRYPTED_KEY must play label for blocked cards', () => {
    render(
      <HandDisplay
        cards={['swap', 'encrypted_key'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={vi.fn()}
        blockedCards={['swap']}
      />,
    )
    expect(screen.getByText('Must play ENCRYPTED_KEY')).toBeInTheDocument()
  })

  it('does not call onSelect for blocked cards', async () => {
    const user = userEvent.setup()
    const onSelect = vi.fn()
    render(
      <HandDisplay
        cards={['swap', 'encrypted_key'] as CardName[]}
        selectedCard={null}
        disabled={false}
        onSelect={onSelect}
        blockedCards={['swap']}
      />,
    )
    // Swap is blocked (disabled), encrypted_key is playable
    const cards = screen.getAllByTestId('card')
    // Click the blocked king — should not fire
    await user.click(cards[0])
    expect(onSelect).not.toHaveBeenCalled()

    // Click countess — should fire
    await user.click(cards[1])
    expect(onSelect).toHaveBeenCalledWith('encrypted_key')
  })
})
