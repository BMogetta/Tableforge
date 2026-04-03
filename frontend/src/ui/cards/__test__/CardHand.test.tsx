import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { CardHand } from '../CardHand'

interface TestCard {
  id: string
  name: string
}

const cards: TestCard[] = [
  { id: '1', name: 'Guard' },
  { id: '2', name: 'Priest' },
  { id: '3', name: 'Baron' },
]

function renderTestHand(overrides = {}) {
  const defaults = {
    cards,
    renderCard: (c: TestCard) => <span>{c.name}</span>,
  }
  return render(<CardHand<TestCard> {...defaults} {...overrides} />)
}

describe('CardHand', () => {
  it('renders all cards', () => {
    renderTestHand()
    expect(screen.getByText('Guard')).toBeInTheDocument()
    expect(screen.getByText('Priest')).toBeInTheDocument()
    expect(screen.getByText('Baron')).toBeInTheDocument()
  })

  it('renders a slot per card with testids', () => {
    renderTestHand()
    expect(screen.getByTestId('card-hand-slot-0')).toBeInTheDocument()
    expect(screen.getByTestId('card-hand-slot-1')).toBeInTheDocument()
    expect(screen.getByTestId('card-hand-slot-2')).toBeInTheDocument()
  })

  it('calls onReveal when faceDown card is clicked', () => {
    const onReveal = vi.fn()
    renderTestHand({ faceDown: true, onReveal })
    const cardButtons = screen.getAllByTestId('card')
    fireEvent.click(cardButtons[1])
    expect(onReveal).toHaveBeenCalledWith(cards[1], 1)
  })

  it('calls onPlay when face-up card is clicked', async () => {
    const onPlay = vi.fn()
    renderTestHand({ onPlay })
    const cardButtons = screen.getAllByTestId('card')
    fireEvent.click(cardButtons[0])
    await waitFor(() => {
      expect(onPlay).toHaveBeenCalledWith(cards[0], 0)
    })
  })

  it('renders empty hand without errors', () => {
    renderTestHand({ cards: [] })
    expect(screen.getByTestId('card-hand')).toBeInTheDocument()
  })

  it('renders single card centered (angle 0)', () => {
    renderTestHand({ cards: [cards[0]] })
    expect(screen.getAllByTestId('card')).toHaveLength(1)
  })
})
