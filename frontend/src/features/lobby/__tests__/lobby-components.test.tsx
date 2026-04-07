import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { RoomCard } from '../components/RoomCard'
import { ActiveGameBanner } from '../components/ActiveGameBanner'
import { RoomCardSkeleton } from '../components/RoomCardSkeleton'
import type { RoomView } from '@/lib/api'

// ---------------------------------------------------------------------------
// RoomCard
// ---------------------------------------------------------------------------

function makeRoomView(overrides?: Partial<RoomView>): RoomView {
  return {
    room: {
      id: 'room-1',
      code: 'ABC123',
      game_id: 'tictactoe',
      owner_id: 'owner-1',
      status: 'waiting',
      max_players: 2,
      created_at: new Date().toISOString(),
      updated_at: new Date().toISOString(),
    },
    players: [{ id: 'p1', username: 'alice', role: 'player', is_bot: false, created_at: new Date().toISOString(), seat: 0, joined_at: new Date().toISOString() }],
    settings: {},
    ...overrides,
  }
}

describe('RoomCard', () => {
  it('renders room code and game id', () => {
    render(<RoomCard view={makeRoomView()} onJoin={vi.fn()} />)
    expect(screen.getByTestId('room-card-code')).toHaveTextContent('ABC123')
    expect(screen.getByText('tictactoe')).toBeInTheDocument()
  })

  it('shows player count', () => {
    render(<RoomCard view={makeRoomView()} onJoin={vi.fn()} />)
    expect(screen.getByText('1/2 players')).toBeInTheDocument()
  })

  it('calls onJoin when join button is clicked', () => {
    const onJoin = vi.fn()
    render(<RoomCard view={makeRoomView()} onJoin={onJoin} />)
    fireEvent.click(screen.getByText('Join →'))
    expect(onJoin).toHaveBeenCalledTimes(1)
  })

  it('disables join button when disabled', () => {
    render(<RoomCard view={makeRoomView()} onJoin={vi.fn()} disabled />)
    expect(screen.getByText('Join →')).toBeDisabled()
  })

  it('shows lock icon for private rooms', () => {
    const view = makeRoomView({ settings: { room_visibility: 'private' } })
    render(<RoomCard view={view} onJoin={vi.fn()} />)
    expect(screen.getByTestId('room-card-private-icon')).toBeInTheDocument()
    expect(screen.queryByText('Join →')).not.toBeInTheDocument()
  })

  it('shows code for public rooms', () => {
    render(<RoomCard view={makeRoomView()} onJoin={vi.fn()} />)
    expect(screen.getByTestId('room-card-code')).toBeInTheDocument()
    expect(screen.queryByTestId('room-card-private-icon')).not.toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// ActiveGameBanner
// ---------------------------------------------------------------------------

describe('ActiveGameBanner', () => {
  const defaults = {
    gameId: 'rootaccess',
    onRejoin: vi.fn(),
    onForfeit: vi.fn(),
    isForfeitPending: false,
  }

  it('renders game id', () => {
    render(<ActiveGameBanner {...defaults} />)
    expect(screen.getByTestId('active-game-banner')).toBeInTheDocument()
    expect(screen.getByText(/rootaccess/)).toBeInTheDocument()
  })

  it('calls onRejoin when rejoin button is clicked', () => {
    const onRejoin = vi.fn()
    render(<ActiveGameBanner {...defaults} onRejoin={onRejoin} />)
    fireEvent.click(screen.getByTestId('rejoin-btn'))
    expect(onRejoin).toHaveBeenCalledTimes(1)
  })

  it('calls onForfeit when forfeit button is clicked', () => {
    const onForfeit = vi.fn()
    render(<ActiveGameBanner {...defaults} onForfeit={onForfeit} />)
    fireEvent.click(screen.getByTestId('forfeit-btn'))
    expect(onForfeit).toHaveBeenCalledTimes(1)
  })

  it('disables forfeit button when pending', () => {
    render(<ActiveGameBanner {...defaults} isForfeitPending={true} />)
    expect(screen.getByTestId('forfeit-btn')).toBeDisabled()
    expect(screen.getByTestId('forfeit-btn')).toHaveTextContent('Forfeiting...')
  })

  it('shows Forfeit text when not pending', () => {
    render(<ActiveGameBanner {...defaults} isForfeitPending={false} />)
    expect(screen.getByTestId('forfeit-btn')).toHaveTextContent('Forfeit')
  })
})

// ---------------------------------------------------------------------------
// RoomCardSkeleton
// ---------------------------------------------------------------------------

describe('RoomCardSkeleton', () => {
  it('renders without crashing', () => {
    const { container } = render(<RoomCardSkeleton />)
    expect(container.querySelector('[aria-hidden="true"]')).toBeInTheDocument()
  })
})
