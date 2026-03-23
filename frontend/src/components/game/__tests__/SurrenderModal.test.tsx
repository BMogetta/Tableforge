import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { SurrenderModal } from '../SurrenderModal'

function renderModal(props?: Partial<{ onConfirm: () => void; onCancel: () => void; isPending: boolean }>) {
  const onConfirm = props?.onConfirm ?? vi.fn()
  const onCancel = props?.onCancel ?? vi.fn()
  const isPending = props?.isPending ?? false
  render(<SurrenderModal onConfirm={onConfirm} onCancel={onCancel} isPending={isPending} />)
  return { onConfirm, onCancel }
}

describe('SurrenderModal', () => {
  it('renders the title and body', () => {
    renderModal()
    expect(screen.getByText('Forfeit game?')).toBeInTheDocument()
    expect(screen.getByText(/you will be recorded as the loser/i)).toBeInTheDocument()
  })

  it('shows "Forfeit" button text when not pending', () => {
    renderModal()
    expect(screen.getByTestId('confirm-surrender-btn')).toHaveTextContent('Forfeit')
  })

  it('shows "Forfeiting..." when pending', () => {
    renderModal({ isPending: true })
    expect(screen.getByTestId('confirm-surrender-btn')).toHaveTextContent('Forfeiting...')
  })

  it('calls onConfirm when Forfeit is clicked', () => {
    const { onConfirm } = renderModal()
    fireEvent.click(screen.getByTestId('confirm-surrender-btn'))
    expect(onConfirm).toHaveBeenCalledTimes(1)
  })

  it('calls onCancel when Cancel is clicked', () => {
    const { onCancel } = renderModal()
    fireEvent.click(screen.getByText('Cancel'))
    expect(onCancel).toHaveBeenCalledTimes(1)
  })

  it('disables both buttons when pending', () => {
    renderModal({ isPending: true })
    expect(screen.getByTestId('confirm-surrender-btn')).toBeDisabled()
    expect(screen.getByText('Cancel')).toBeDisabled()
  })

  it('does not call onConfirm when button is disabled', () => {
    const { onConfirm } = renderModal({ isPending: true })
    fireEvent.click(screen.getByTestId('confirm-surrender-btn'))
    expect(onConfirm).not.toHaveBeenCalled()
  })
})