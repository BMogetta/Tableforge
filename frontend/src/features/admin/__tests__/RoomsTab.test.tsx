import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { RoomsTab } from '../components/RoomsTab'
import type { Room } from '@/lib/schema-generated.zod'

const { mockToast, mockAdmin } = vi.hoisted(() => ({
  mockToast: { showError: vi.fn(), showWarning: vi.fn(), showInfo: vi.fn() },
  mockAdmin: {
    listRooms: vi.fn(),
    getRoom: vi.fn(),
    forceEndSession: vi.fn(),
  },
}))

vi.mock('@/ui/Toast', () => ({ useToast: () => mockToast }))
vi.mock('@/features/admin/api', () => ({ admin: mockAdmin }))

const ROOMS: Room[] = [
  {
    id: 'room1',
    code: 'ABC123',
    game_id: 'tictactoe',
    owner_id: 'p1',
    status: 'waiting',
    max_players: 2,
    created_at: '2025-01-01T00:00:00Z',
    updated_at: '2025-01-01T00:00:00Z',
  },
  {
    id: 'room2',
    code: 'XYZ789',
    game_id: 'rootaccess',
    owner_id: 'p2',
    status: 'in_progress',
    max_players: 4,
    created_at: '2025-01-02T00:00:00Z',
    updated_at: '2025-01-02T00:00:00Z',
  },
]

describe('RoomsTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAdmin.listRooms.mockResolvedValue(ROOMS)
    window.confirm = vi.fn().mockReturnValue(true)
  })

  it('renders rooms table', async () => {
    render(<RoomsTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('rooms-table')).toBeInTheDocument())
    expect(screen.getByText('ABC123')).toBeInTheDocument()
    expect(screen.getByText('XYZ789')).toBeInTheDocument()
  })

  it('shows empty state when no rooms', async () => {
    mockAdmin.listRooms.mockResolvedValue([])
    render(<RoomsTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('rooms-empty')).toBeInTheDocument())
  })

  it('filters rooms by status', async () => {
    render(<RoomsTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('rooms-table')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('rooms-filter-waiting'))
    expect(screen.getByText('ABC123')).toBeInTheDocument()
    expect(screen.queryByText('XYZ789')).not.toBeInTheDocument()
  })

  it('opens room detail on row click', async () => {
    mockAdmin.getRoom.mockResolvedValue({
      room: ROOMS[0],
      players: [
        { id: 'p1', username: 'alice', role: 'player', is_bot: false, seat: 0, joined_at: '2025-01-01T00:00:00Z', created_at: '2025-01-01T00:00:00Z' },
      ],
      settings: {},
    })
    render(<RoomsTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('room-row-room1')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('room-row-room1'))
    await waitFor(() => expect(screen.getByTestId('room-detail')).toBeInTheDocument())
    expect(screen.getByText('alice')).toBeInTheDocument()
  })

  it('shows force-end button for in_progress rooms when manager', async () => {
    mockAdmin.getRoom.mockResolvedValue({
      room: ROOMS[1],
      players: [],
      settings: {},
    })
    render(<RoomsTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('room-row-room2')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('room-row-room2'))
    await waitFor(() => expect(screen.getByTestId('force-end-btn')).toBeInTheDocument())
  })

  it('calls forceEndSession on confirm', async () => {
    mockAdmin.forceEndSession.mockResolvedValue(undefined)
    mockAdmin.getRoom.mockResolvedValue({
      room: ROOMS[1],
      players: [],
      settings: {},
    })
    render(<RoomsTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('room-row-room2')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('room-row-room2'))
    await waitFor(() => expect(screen.getByTestId('force-end-btn')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('force-end-btn'))
    await waitFor(() => expect(mockAdmin.forceEndSession).toHaveBeenCalledWith('room2'))
  })
})
