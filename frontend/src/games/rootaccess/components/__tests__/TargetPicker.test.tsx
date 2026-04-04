import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { TargetPicker } from '../TargetPicker'
import type { CardName } from '../CardDisplay'

const opponents = [
  { id: 'p2', username: 'Player 2' },
  { id: 'p3', username: 'Player 3' },
]

describe('TargetPicker', () => {
  it('renders all opponents', () => {
    render(
      <TargetPicker
        opponents={opponents}
        eliminatedIds={[]}
        protectedIds={[]}
        selectedTarget={null}
        cardBeingPlayed={'ping' as CardName}
        selectedGuess={null}
        onSelectTarget={vi.fn()}
        onSelectGuess={vi.fn()}
      />,
    )
    expect(screen.getByText('Player 2')).toBeInTheDocument()
    expect(screen.getByText('Player 3')).toBeInTheDocument()
  })

  it('calls onSelectTarget when a valid target is clicked', async () => {
    const user = userEvent.setup()
    const onSelectTarget = vi.fn()
    render(
      <TargetPicker
        opponents={opponents}
        eliminatedIds={[]}
        protectedIds={[]}
        selectedTarget={null}
        cardBeingPlayed={'sniffer' as CardName}
        selectedGuess={null}
        onSelectTarget={onSelectTarget}
        onSelectGuess={vi.fn()}
      />,
    )
    await user.click(screen.getByText('Player 2').closest('button')!)
    expect(onSelectTarget).toHaveBeenCalledWith('p2')
  })

  it('disables eliminated players', () => {
    render(
      <TargetPicker
        opponents={opponents}
        eliminatedIds={['p2']}
        protectedIds={[]}
        selectedTarget={null}
        cardBeingPlayed={'buffer_overflow' as CardName}
        selectedGuess={null}
        onSelectTarget={vi.fn()}
        onSelectGuess={vi.fn()}
      />,
    )
    const p2Btn = screen.getByText('Player 2').closest('button')!
    expect(p2Btn).toBeDisabled()
    expect(screen.getByText('Eliminated')).toBeInTheDocument()
  })

  it('disables protected players', () => {
    render(
      <TargetPicker
        opponents={opponents}
        eliminatedIds={[]}
        protectedIds={['p3']}
        selectedTarget={null}
        cardBeingPlayed={'swap' as CardName}
        selectedGuess={null}
        onSelectTarget={vi.fn()}
        onSelectGuess={vi.fn()}
      />,
    )
    const p3Btn = screen.getByText('Player 3').closest('button')!
    expect(p3Btn).toBeDisabled()
    expect(screen.getByText('Protected')).toBeInTheDocument()
  })

  it('shows no-effect hint when all opponents are unavailable', () => {
    render(
      <TargetPicker
        opponents={opponents}
        eliminatedIds={['p2']}
        protectedIds={['p3']}
        selectedTarget={null}
        cardBeingPlayed={'ping' as CardName}
        selectedGuess={null}
        onSelectTarget={vi.fn()}
        onSelectGuess={vi.fn()}
      />,
    )
    expect(screen.getByText(/no effect/i)).toBeInTheDocument()
  })

  it('shows ping guess dropdown only when card is ping', () => {
    const { rerender } = render(
      <TargetPicker
        opponents={opponents}
        eliminatedIds={[]}
        protectedIds={[]}
        selectedTarget={null}
        cardBeingPlayed={'ping' as CardName}
        selectedGuess={null}
        onSelectTarget={vi.fn()}
        onSelectGuess={vi.fn()}
      />,
    )
    expect(screen.getByText('Guess their card')).toBeInTheDocument()

    rerender(
      <TargetPicker
        opponents={opponents}
        eliminatedIds={[]}
        protectedIds={[]}
        selectedTarget={null}
        cardBeingPlayed={'sniffer' as CardName}
        selectedGuess={null}
        onSelectTarget={vi.fn()}
        onSelectGuess={vi.fn()}
      />,
    )
    expect(screen.queryByText('Guess their card')).not.toBeInTheDocument()
  })

  it('calls onSelectGuess when ping guess is changed', async () => {
    const user = userEvent.setup()
    const onSelectGuess = vi.fn()
    render(
      <TargetPicker
        opponents={opponents}
        eliminatedIds={[]}
        protectedIds={[]}
        selectedTarget={'p2'}
        cardBeingPlayed={'ping' as CardName}
        selectedGuess={null}
        onSelectTarget={vi.fn()}
        onSelectGuess={onSelectGuess}
      />,
    )
    await user.selectOptions(screen.getByRole('combobox'), 'sniffer')
    expect(onSelectGuess).toHaveBeenCalledWith('sniffer')
  })

  it('does not include ping in guess options', () => {
    render(
      <TargetPicker
        opponents={opponents}
        eliminatedIds={[]}
        protectedIds={[]}
        selectedTarget={null}
        cardBeingPlayed={'ping' as CardName}
        selectedGuess={null}
        onSelectTarget={vi.fn()}
        onSelectGuess={vi.fn()}
      />,
    )
    const options = screen.getAllByRole('option')
    const optionValues = options.map(o => o.getAttribute('value'))
    expect(optionValues).not.toContain('ping')
  })
})
