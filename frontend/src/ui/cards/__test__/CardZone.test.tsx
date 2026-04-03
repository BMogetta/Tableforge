import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { CardZone } from '../CardZone'
import type { CardZoneEntry } from '../types'

interface TestCard {
  name: string
}

const entries: CardZoneEntry<TestCard>[] = [
  { key: 'a', data: { name: 'Guard' } },
  { key: 'b', data: { name: 'Priest' }, faceDown: true },
]

function renderTestZone(overrides = {}) {
  const defaults = {
    cards: entries,
    renderCard: (c: TestCard) => <span>{c.name}</span>,
  }
  return render(<CardZone<TestCard> {...defaults} {...overrides} />)
}

describe('CardZone', () => {
  it('renders all cards', () => {
    renderTestZone()
    expect(screen.getByText('Guard')).toBeInTheDocument()
    expect(screen.getByText('Priest')).toBeInTheDocument()
  })

  it('renders with pile layout by default', () => {
    const { container } = renderTestZone()
    expect(container.querySelector('[class*="pile"]')).toBeInTheDocument()
  })

  it('renders with spread layout', () => {
    const { container } = renderTestZone({ layout: 'spread' })
    expect(container.querySelector('[class*="spread"]')).toBeInTheDocument()
  })

  it('renders empty zone without errors', () => {
    renderTestZone({ cards: [] })
    expect(screen.getByTestId('card-zone')).toBeInTheDocument()
  })

  it('respects per-card faceDown state', () => {
    renderTestZone()
    const cards = screen.getAllByTestId('card')
    // First card is face-up
    expect(cards[0]).toHaveAttribute('aria-label', 'Face-up card')
    // Second card has faceDown: true
    expect(cards[1]).toHaveAttribute('aria-label', 'Face-down card')
  })
})
