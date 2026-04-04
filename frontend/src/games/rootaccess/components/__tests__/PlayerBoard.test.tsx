import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PlayerBoard } from '../PlayerBoard'
import type { CardName } from '../CardDisplay'

const baseProps = {
  playerId: 'p1',
  username: 'Player 1',
  tokens: 0,
  tokensToWin: 7,
  discardPile: [] as CardName[],
  isEliminated: false,
  isProtected: false,
  hasPlayedBackdoor: false,
  isLocal: false,
  isCurrentTurn: false,
}

describe('PlayerBoard', () => {
  it('renders username', () => {
    render(<PlayerBoard {...baseProps} />)
    expect(screen.getByText('Player 1')).toBeInTheDocument()
  })

  it('renders "you" badge for local player', () => {
    render(<PlayerBoard {...baseProps} isLocal={true} />)
    expect(screen.getByText('you')).toBeInTheDocument()
  })

  it('does not render "you" badge for non-local player', () => {
    render(<PlayerBoard {...baseProps} />)
    expect(screen.queryByText('you')).not.toBeInTheDocument()
  })

  it('renders turn indicator when it is current turn', () => {
    render(<PlayerBoard {...baseProps} isCurrentTurn={true} />)
    expect(screen.getByText('▶')).toBeInTheDocument()
  })

  it('does not render turn indicator when not current turn', () => {
    render(<PlayerBoard {...baseProps} />)
    expect(screen.queryByText('▶')).not.toBeInTheDocument()
  })

  it('renders correct number of filled tokens', () => {
    render(<PlayerBoard {...baseProps} tokens={3} tokensToWin={7} />)
    const filled = document.querySelectorAll('[aria-label="Token earned"]')
    const pending = document.querySelectorAll('[aria-label="Token pending"]')
    expect(filled).toHaveLength(3)
    expect(pending).toHaveLength(4)
  })

  it('renders Protected badge when protected', () => {
    render(<PlayerBoard {...baseProps} isProtected={true} />)
    expect(screen.getByText('Shielded')).toBeInTheDocument()
  })

  it('does not render Protected badge when not protected', () => {
    render(<PlayerBoard {...baseProps} />)
    expect(screen.queryByText('Shielded')).not.toBeInTheDocument()
  })

  it('renders Eliminated badge when eliminated', () => {
    render(<PlayerBoard {...baseProps} isEliminated={true} />)
    expect(screen.getByText('Eliminated')).toBeInTheDocument()
  })

  it('does not render Eliminated badge when not eliminated', () => {
    render(<PlayerBoard {...baseProps} />)
    expect(screen.queryByText('Eliminated')).not.toBeInTheDocument()
  })

  it('renders Backdoor badge when player has played backdoor', () => {
    render(<PlayerBoard {...baseProps} hasPlayedBackdoor={true} />)
    expect(screen.getByText('Backdoor')).toBeInTheDocument()
  })

  it('renders discard pile cards', () => {
    render(<PlayerBoard {...baseProps} discardPile={['ping', 'sniffer'] as CardName[]} />)
    expect(screen.getByText('PING')).toBeInTheDocument()
    expect(screen.getByText('SNIFFER')).toBeInTheDocument()
  })

  it('renders empty discard indicator when pile is empty', () => {
    render(<PlayerBoard {...baseProps} discardPile={[]} />)
    expect(screen.getByText('—')).toBeInTheDocument()
  })
})
