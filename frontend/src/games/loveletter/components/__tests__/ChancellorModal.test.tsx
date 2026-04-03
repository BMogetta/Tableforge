import { describe, it, expect, vi } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { ChancellorModal } from '../ChancellorModal'
import type { CardName } from '../CardDisplay'

const choices: CardName[] = ['spy', 'guard', 'priest']

describe('ChancellorModal', () => {
  it('renders all 3 choices', () => {
    render(<ChancellorModal choices={choices} onConfirm={vi.fn()} />)
    expect(screen.getByText('Spy')).toBeInTheDocument()
    expect(screen.getByText('Guard')).toBeInTheDocument()
    expect(screen.getByText('Priest')).toBeInTheDocument()
  })

  it('Confirm button is disabled until keep and return order are selected', async () => {
    const user = userEvent.setup()
    render(<ChancellorModal choices={choices} onConfirm={vi.fn()} />)

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
    render(<ChancellorModal choices={choices} onConfirm={onConfirm} />)

    // Keep spy (index 0).
    const allCards = screen.getAllByTestId('card')
    await user.click(allCards[0])

    // Select return order: guard first, priest second.
    const returnCards = screen.getAllByTestId('card')
    await user.click(returnCards[3])
    await user.click(returnCards[4])

    await user.click(screen.getByRole('button', { name: /confirm/i }))

    expect(onConfirm).toHaveBeenCalledOnce()
    expect(onConfirm).toHaveBeenCalledWith('spy', ['guard', 'priest'])
  })

  it('shows Keep tag on selected keep card', async () => {
    const user = userEvent.setup()
    render(<ChancellorModal choices={choices} onConfirm={vi.fn()} />)

    const cards = screen.getAllByTestId('card')
    await user.click(cards[0])

    expect(screen.getByText('Keep')).toBeInTheDocument()
  })

  it('shows return order tags on selected return cards', async () => {
    const user = userEvent.setup()
    render(<ChancellorModal choices={choices} onConfirm={vi.fn()} />)

    const allCards = screen.getAllByTestId('card')
    await user.click(allCards[0]) // keep spy

    const returnCards = screen.getAllByTestId('card')
    await user.click(returnCards[3]) // guard → 1
    await user.click(returnCards[4]) // priest → 2

    expect(screen.getAllByTestId('return-order-1')).toHaveLength(2)
    expect(screen.getAllByTestId('return-order-2')).toHaveLength(2)
  })

  it('deselects a return card when clicked again', async () => {
    const user = userEvent.setup()
    render(<ChancellorModal choices={choices} onConfirm={vi.fn()} />)

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
