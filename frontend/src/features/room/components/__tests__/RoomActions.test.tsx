import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { RoomActions } from '../RoomActions'

const base = {
  isSpectator: false,
  isOwner: true,
  canStart: true,
  starting: false,
  startError: null,
  playersNeeded: 0,
  onStart: vi.fn(),
  onLeave: vi.fn(),
}

describe('RoomActions', () => {
  it('shows Start Game button for owner with enough players', () => {
    render(<RoomActions {...base} />)
    expect(screen.getByTestId('start-game-btn')).toHaveTextContent('Start Game')
  })

  it('shows need more players message when not enough', () => {
    render(<RoomActions {...base} canStart={false} playersNeeded={1} />)
    expect(screen.getByTestId('start-game-btn')).toHaveTextContent('Need 1 more player(s)')
  })

  it('shows Starting... when starting', () => {
    render(<RoomActions {...base} starting={true} />)
    expect(screen.getByTestId('start-game-btn')).toHaveTextContent('Starting...')
    expect(screen.getByTestId('start-game-btn')).toBeDisabled()
  })

  it('calls onStart when clicking start', () => {
    const onStart = vi.fn()
    render(<RoomActions {...base} onStart={onStart} />)
    fireEvent.click(screen.getByTestId('start-game-btn'))
    expect(onStart).toHaveBeenCalledTimes(1)
  })

  it('calls onLeave when clicking leave (owner)', () => {
    const onLeave = vi.fn()
    render(<RoomActions {...base} onLeave={onLeave} />)
    fireEvent.click(screen.getByText('Leave'))
    expect(onLeave).toHaveBeenCalledTimes(1)
  })

  it('shows waiting message for non-owner participant', () => {
    render(<RoomActions {...base} isOwner={false} />)
    expect(screen.getByText(/waiting for host/i)).toBeInTheDocument()
  })

  it('shows only leave button for spectator', () => {
    render(<RoomActions {...base} isSpectator={true} />)
    expect(screen.getByText('Leave')).toBeInTheDocument()
    expect(screen.queryByTestId('start-game-btn')).not.toBeInTheDocument()
  })
})
