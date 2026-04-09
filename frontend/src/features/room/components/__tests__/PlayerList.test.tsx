import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import { PlayerList } from '../PlayerList'
import type { RoomPlayer } from '@/lib/schema-generated.zod'

vi.mock('@/features/room/api', () => ({
  rooms: {},
  mutes: { mute: vi.fn(), unmute: vi.fn() },
}))

vi.mock('@/features/friends/api', () => ({
  friends: { sendRequest: vi.fn() },
}))

vi.mock('@/ui/Toast', () => ({
  useToast: () => ({ showInfo: vi.fn(), showError: vi.fn() }),
}))

vi.mock('@/stores/store', () => ({
  useAppStore: Object.assign(
    (selector: (s: unknown) => unknown) => selector({ presenceMap: { 'p1': true, 'p2': false } }),
    { getState: () => ({ setDmTarget: vi.fn() }) },
  ),
}))

const players: RoomPlayer[] = [
  { id: 'p1', username: 'Alice', is_bot: false, seat: 0 },
  { id: 'p2', username: 'Bob', is_bot: false, seat: 1 },
]

const botPlayer: RoomPlayer = { id: 'bot1', username: 'BotCarl', is_bot: true, seat: 2 }

const baseProps = {
  players,
  maxPlayers: 4,
  ownerId: 'p1',
  currentPlayerId: 'p1',
  isOwner: true,
  mutedIds: new Set<string>(),
  spectatorCount: 0,
  removingBotId: null,
  onMute: vi.fn(),
  onUnmute: vi.fn(),
  onRemoveBot: vi.fn(),
}

beforeEach(() => {
  vi.clearAllMocks()
})

describe('PlayerList', () => {
  it('renders player names', () => {
    render(<PlayerList {...baseProps} />)
    expect(screen.getByText('Alice')).toBeInTheDocument()
    expect(screen.getByText('Bob')).toBeInTheDocument()
  })

  it('shows host badge for owner', () => {
    render(<PlayerList {...baseProps} />)
    expect(screen.getByText(/host/i)).toBeInTheDocument()
  })

  it('shows "you" badge for current player', () => {
    render(<PlayerList {...baseProps} />)
    expect(screen.getByText(/you/i)).toBeInTheDocument()
  })

  it('renders empty slots', () => {
    render(<PlayerList {...baseProps} />)
    // 4 max - 2 players = 2 empty slots
    const waitingTexts = screen.getAllByText(/waiting/i)
    expect(waitingTexts).toHaveLength(2)
  })

  it('shows bot badge and remove button for bot players', () => {
    render(
      <PlayerList {...baseProps} players={[...players, botPlayer]} />,
    )
    expect(screen.getByText('Bot')).toBeInTheDocument()
    expect(screen.getByTitle('Remove bot')).toBeInTheDocument()
  })

  it('hides remove bot button for non-owners', () => {
    render(
      <PlayerList
        {...baseProps}
        players={[...players, botPlayer]}
        isOwner={false}
        currentPlayerId='p2'
      />,
    )
    expect(screen.queryByTitle('Remove bot')).toBeNull()
  })

  it('shows spectator count when > 0', () => {
    render(<PlayerList {...baseProps} spectatorCount={3} />)
    expect(screen.getByText(/3/)).toBeInTheDocument()
  })

  it('hides spectator count when 0', () => {
    const { container } = render(<PlayerList {...baseProps} spectatorCount={0} />)
    expect(container.querySelector('[data-testid="spectator-count"]')).toBeNull()
  })

  it('disables remove bot button when bot is being removed', () => {
    render(
      <PlayerList
        {...baseProps}
        players={[...players, botPlayer]}
        removingBotId='bot1'
      />,
    )
    expect(screen.getByTitle('Remove bot')).toBeDisabled()
  })
})
