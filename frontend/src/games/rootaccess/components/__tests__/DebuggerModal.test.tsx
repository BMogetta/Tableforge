import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DebuggerModal } from '../DebuggerModal'
import type { CardName } from '../CardDisplay'

const choices: CardName[] = ['backdoor', 'ping', 'sniffer']

describe('DebuggerModal', () => {
  it('renders all 3 choices', () => {
    render(<DebuggerModal choices={choices} onConfirm={vi.fn()} />)
    expect(screen.getByText('BACKDOOR')).toBeInTheDocument()
    expect(screen.getByText('PING')).toBeInTheDocument()
    expect(screen.getByText('SNIFFER')).toBeInTheDocument()
  })

  it('Confirm button is disabled until keep and return order are selected', async () => {
    const user = userEvent.setup()
    render(<DebuggerModal choices={choices} onConfirm={vi.fn()} />)

    const confirmBtn = screen.getByRole('button', { name: /confirm/i })
    expect(confirmBtn).toBeDisabled()

    // Select keep card — click spy (first card).
    const allCards = screen.getAllByTestId('card')
    await user.click(allCards[0])
    expect(confirmBtn).toBeDisabled()

    // Return order section now visible — get all cards again.
    const returnCards = screen.getAllByTestId('card')
    // returnCards: [spy, guard, priest] from "Your choices" + [guard, priest] from "Return order"
    // Return order cards are at indices 3 and 4.
    await user.click(returnCards[3])
    expect(confirmBtn).toBeDisabled()

    await user.click(returnCards[4])
    expect(confirmBtn).not.toBeDisabled()
  })

  it('calls onConfirm with correct keep and return args', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    render(<DebuggerModal choices={choices} onConfirm={onConfirm} />)

    // Keep spy (index 0).
    const allCards = screen.getAllByTestId('card')
    await user.click(allCards[0])

    // Select return order: guard first, priest second.
    const returnCards = screen.getAllByTestId('card')
    await user.click(returnCards[3])
    await user.click(returnCards[4])

    await user.click(screen.getByRole('button', { name: /confirm/i }))

    expect(onConfirm).toHaveBeenCalledOnce()
    expect(onConfirm).toHaveBeenCalledWith('backdoor', ['ping', 'sniffer'])
  })

  it('shows Keep tag on selected keep card', async () => {
    const user = userEvent.setup()
    render(<DebuggerModal choices={choices} onConfirm={vi.fn()} />)

    const cards = screen.getAllByTestId('card')
    await user.click(cards[0])

    expect(screen.getByText('Keep')).toBeInTheDocument()
  })

  it('shows return order tags on selected return cards', async () => {
    const user = userEvent.setup()
    render(<DebuggerModal choices={choices} onConfirm={vi.fn()} />)

    const allCards = screen.getAllByTestId('card')
    await user.click(allCards[0]) // keep spy

    const returnCards = screen.getAllByTestId('card')
    await user.click(returnCards[3]) // guard → 1
    await user.click(returnCards[4]) // priest → 2

    expect(screen.getAllByTestId('return-order-1')).toHaveLength(2)
    expect(screen.getAllByTestId('return-order-2')).toHaveLength(2)
  })

  it('handles duplicate choices: independent selection per copy', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    const dupChoices: CardName[] = ['ping', 'ping', 'sniffer']
    render(<DebuggerModal choices={dupChoices} onConfirm={onConfirm} />)

    // Keep sniffer (index 2) — the distinct one.
    const allCards = screen.getAllByTestId('card')
    await user.click(allCards[2])

    // Return section shows both identical pings. Click each independently.
    const returnCards = screen.getAllByTestId('card')
    // After keep: [ping, ping, sniffer] in top + [ping, ping] in return section.
    // Return cards live at indices 3 and 4.
    await user.click(returnCards[3]) // first ping → position 1
    await user.click(returnCards[4]) // second ping → position 2

    // Both positions must be assigned — not collapsed by value equality.
    expect(screen.getAllByTestId('return-order-1')).toHaveLength(2)
    expect(screen.getAllByTestId('return-order-2')).toHaveLength(2)

    await user.click(screen.getByRole('button', { name: /confirm/i }))
    expect(onConfirm).toHaveBeenCalledWith('sniffer', ['ping', 'ping'])
  })

  it('handles duplicate choices: keeping one of two identicals leaves the other available', async () => {
    const user = userEvent.setup()
    const onConfirm = vi.fn()
    const dupChoices: CardName[] = ['ping', 'ping', 'sniffer']
    render(<DebuggerModal choices={dupChoices} onConfirm={onConfirm} />)

    // Keep the first ping (index 0). The other ping + sniffer must remain.
    const allCards = screen.getAllByTestId('card')
    await user.click(allCards[0])

    // Return section: exactly 2 cards (the other ping + sniffer), not 1.
    // Total cards = 3 (top) + 2 (return) = 5.
    const returnCards = screen.getAllByTestId('card')
    expect(returnCards).toHaveLength(5)

    await user.click(returnCards[3]) // other ping → position 1
    await user.click(returnCards[4]) // sniffer → position 2

    await user.click(screen.getByRole('button', { name: /confirm/i }))
    expect(onConfirm).toHaveBeenCalledWith('ping', ['ping', 'sniffer'])
  })

  it('deselects a return card when clicked again', async () => {
    const user = userEvent.setup()
    render(<DebuggerModal choices={choices} onConfirm={vi.fn()} />)

    const allCards = screen.getAllByTestId('card')
    await user.click(allCards[0]) // keep spy

    const returnCards = screen.getAllByTestId('card')
    await user.click(returnCards[3]) // select guard as return 1
    expect(screen.getAllByTestId('return-order-1')).toHaveLength(2)

    await user.click(returnCards[3]) // deselect
    expect(screen.queryByTestId('return-order-1')).not.toBeInTheDocument()

    expect(screen.getByRole('button', { name: /confirm/i })).toBeDisabled()
  })
})
