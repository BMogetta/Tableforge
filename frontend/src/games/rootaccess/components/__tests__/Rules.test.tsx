import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import { CARD_META, type CardName } from '../CardDisplay'
import { RootAccessRules } from '../Rules'

describe('RootAccessRules', () => {
  it('renders all sections', () => {
    render(<RootAccessRules />)
    expect(screen.getByText('Objective')).toBeInTheDocument()
    expect(screen.getByText('Turn Flow')).toBeInTheDocument()
    expect(screen.getByText('Win Conditions')).toBeInTheDocument()
    expect(screen.getByText('Card Reference')).toBeInTheDocument()
    expect(screen.getByText('Special Rules')).toBeInTheDocument()
  })

  it('renders all 10 cards in the reference table', () => {
    render(<RootAccessRules />)
    const allCards = Object.keys(CARD_META) as CardName[]
    for (const card of allCards) {
      expect(screen.getAllByText(CARD_META[card].label).length).toBeGreaterThanOrEqual(1)
    }
  })

  it('renders card effects', () => {
    render(<RootAccessRules />)
    expect(screen.getByText(CARD_META.ping.effect)).toBeInTheDocument()
    expect(screen.getByText(CARD_META.root.effect)).toBeInTheDocument()
  })

  it('renders tokens-to-win table', () => {
    render(<RootAccessRules />)
    expect(screen.getByText('Tokens to win')).toBeInTheDocument()
  })

  it('highlights cards in hand', () => {
    const { container } = render(<RootAccessRules handCards={['ping', 'root']} />)
    const highlightedRows = container.querySelectorAll('[class*="inHand"]')
    expect(highlightedRows.length).toBe(2)
  })

  it('does not highlight cards when handCards is empty', () => {
    const { container } = render(<RootAccessRules handCards={[]} />)
    const highlightedRows = container.querySelectorAll('[class*="inHand"]')
    expect(highlightedRows.length).toBe(0)
  })
})
