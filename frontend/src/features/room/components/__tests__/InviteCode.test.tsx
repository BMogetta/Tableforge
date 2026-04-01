import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { InviteCode } from '../InviteCode'

describe('InviteCode', () => {
  it('shows the room code', () => {
    render(<InviteCode code='ABC123' isPrivate={false} />)
    expect(screen.getByTestId('room-code-display')).toHaveTextContent('ABC123')
  })

  it('shows "Invite Code" label for public rooms', () => {
    render(<InviteCode code='ABC123' isPrivate={false} />)
    expect(screen.getByText('Invite Code')).toBeInTheDocument()
  })

  it('shows "Private Room" label for private rooms', () => {
    render(<InviteCode code='XYZ789' isPrivate={true} />)
    expect(screen.getByText('Private Room')).toBeInTheDocument()
    expect(screen.getByText(/share the room code privately/i)).toBeInTheDocument()
  })

  it('copies code to clipboard on click', () => {
    const writeText = vi.fn().mockResolvedValue(undefined)
    Object.assign(navigator, { clipboard: { writeText } })

    render(<InviteCode code='COPY_ME' isPrivate={false} />)
    fireEvent.click(screen.getByText('Copy'))
    expect(writeText).toHaveBeenCalledWith('COPY_ME')
  })
})
