import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { PlayersTab } from '../components/PlayersTab'
import type { Player } from '@/lib/schema-generated.zod'

const { mockToast, mockAdmin } = vi.hoisted(() => ({
  mockToast: { showError: vi.fn(), showWarning: vi.fn(), showInfo: vi.fn() },
  mockAdmin: {
    listPlayers: vi.fn(),
    setRole: vi.fn(),
  },
}))

vi.mock('@/ui/Toast', () => ({ useToast: () => mockToast }))
vi.mock('@/features/admin/api', () => ({ admin: mockAdmin }))

const PLAYERS: Player[] = [
  { id: 'p1', username: 'alice', role: 'player', is_bot: false, created_at: '2025-01-01T00:00:00Z' },
  { id: 'p2', username: 'bob', role: 'manager', is_bot: false, created_at: '2025-01-02T00:00:00Z' },
  { id: 'p3', username: 'admin', role: 'owner', is_bot: false, created_at: '2025-01-03T00:00:00Z' },
]

describe('PlayersTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAdmin.listPlayers.mockResolvedValue(PLAYERS)
  })

  it('renders player table', async () => {
    render(<PlayersTab callerRole='owner' callerID='p3' />)
    await waitFor(() => expect(screen.getByTestId('players-table')).toBeInTheDocument())
    expect(screen.getByText('alice')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
  })

  it('shows "you" badge for the caller', async () => {
    render(<PlayersTab callerRole='owner' callerID='p3' />)
    await waitFor(() => expect(screen.getByText('you')).toBeInTheDocument())
  })

  it('shows role select for owners, not for managers', async () => {
    const { unmount } = render(<PlayersTab callerRole='owner' callerID='p3' />)
    await waitFor(() => expect(screen.getByTestId('players-table')).toBeInTheDocument())
    expect(screen.getByTestId('role-select-p1')).toBeInTheDocument()
    unmount()

    mockAdmin.listPlayers.mockResolvedValue(PLAYERS)
    render(<PlayersTab callerRole='manager' callerID='p2' />)
    await waitFor(() => expect(screen.getByTestId('players-table')).toBeInTheDocument())
    expect(screen.queryByTestId('role-select-p1')).not.toBeInTheDocument()
  })

  it('calls setRole when role is changed', async () => {
    mockAdmin.setRole.mockResolvedValue(undefined)
    render(<PlayersTab callerRole='owner' callerID='p3' />)
    await waitFor(() => expect(screen.getByTestId('role-select-p1')).toBeInTheDocument())

    fireEvent.change(screen.getByTestId('role-select-p1'), { target: { value: 'manager' } })
    await waitFor(() => expect(mockAdmin.setRole).toHaveBeenCalledWith('p1', 'manager'))
  })

  it('shows toast on error', async () => {
    mockAdmin.listPlayers.mockRejectedValue({ status: 500, message: 'Server error' })
    render(<PlayersTab callerRole='owner' callerID='p3' />)
    await waitFor(() => expect(mockToast.showError).toHaveBeenCalled())
  })
})
