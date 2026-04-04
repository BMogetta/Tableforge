import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { BansTab } from '../components/BansTab'
import type { Ban } from '@/lib/schema-generated.zod'

const { mockToast, mockAdmin } = vi.hoisted(() => ({
  mockToast: { showError: vi.fn(), showWarning: vi.fn(), showInfo: vi.fn() },
  mockAdmin: {
    listPlayerBans: vi.fn(),
    banPlayer: vi.fn(),
    liftBan: vi.fn(),
  },
}))

vi.mock('@/ui/Toast', () => ({ useToast: () => mockToast }))
vi.mock('@/features/admin/api', () => ({ admin: mockAdmin }))

const ACTIVE_BAN: Ban = {
  id: 'b1',
  player_id: 'p1',
  banned_by: 'admin',
  reason: 'Toxic behavior',
  expires_at: new Date(Date.now() + 86_400_000 * 30).toISOString(),
  created_at: '2025-01-15T00:00:00Z',
}

const EXPIRED_BAN: Ban = {
  id: 'b2',
  player_id: 'p1',
  banned_by: 'admin',
  reason: 'Spam',
  lifted_at: '2025-01-20T00:00:00Z',
  created_at: '2025-01-10T00:00:00Z',
}

describe('BansTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAdmin.listPlayerBans.mockResolvedValue([ACTIVE_BAN, EXPIRED_BAN])
    window.confirm = vi.fn().mockReturnValue(true)
  })

  it('shows initial empty state before search', () => {
    render(<BansTab callerRole='manager' />)
    expect(screen.getByTestId('bans-empty')).toBeInTheDocument()
  })

  it('searches and shows active bans', async () => {
    render(<BansTab callerRole='manager' />)
    fireEvent.change(screen.getByTestId('bans-search-input'), { target: { value: 'p1' } })
    fireEvent.click(screen.getByTestId('bans-search-btn'))
    await waitFor(() => expect(screen.getByTestId('active-bans-table')).toBeInTheDocument())
    expect(screen.getByText('Toxic behavior')).toBeInTheDocument()
  })

  it('shows history section with expired bans', async () => {
    render(<BansTab callerRole='manager' />)
    fireEvent.change(screen.getByTestId('bans-search-input'), { target: { value: 'p1' } })
    fireEvent.click(screen.getByTestId('bans-search-btn'))
    await waitFor(() => expect(screen.getByTestId('ban-history-toggle')).toBeInTheDocument())
  })

  it('shows no results when player has no bans', async () => {
    mockAdmin.listPlayerBans.mockResolvedValue([])
    render(<BansTab callerRole='manager' />)
    fireEvent.change(screen.getByTestId('bans-search-input'), { target: { value: 'p99' } })
    fireEvent.click(screen.getByTestId('bans-search-btn'))
    await waitFor(() => expect(screen.getByTestId('bans-no-results')).toBeInTheDocument())
  })

  it('opens ban dialog on button click', async () => {
    render(<BansTab callerRole='manager' />)
    fireEvent.click(screen.getByTestId('open-ban-dialog-btn'))
    expect(screen.getByTestId('ban-dialog')).toBeInTheDocument()
  })

  it('submits ban from dialog', async () => {
    const newBan: Ban = {
      id: 'b3',
      player_id: 'p5',
      banned_by: 'admin',
      reason: 'Test ban',
      created_at: new Date().toISOString(),
    }
    mockAdmin.banPlayer.mockResolvedValue(newBan)
    render(<BansTab callerRole='manager' />)
    fireEvent.click(screen.getByTestId('open-ban-dialog-btn'))

    fireEvent.change(screen.getByTestId('ban-player-id-input'), { target: { value: 'p5' } })
    fireEvent.change(screen.getByTestId('ban-reason-input'), { target: { value: 'Test ban' } })
    fireEvent.click(screen.getByTestId('confirm-ban-btn'))

    await waitFor(() =>
      expect(mockAdmin.banPlayer).toHaveBeenCalledWith('p5', 'Test ban', expect.any(String)),
    )
  })

  it('lifts a ban', async () => {
    mockAdmin.liftBan.mockResolvedValue(undefined)
    render(<BansTab callerRole='manager' />)
    fireEvent.change(screen.getByTestId('bans-search-input'), { target: { value: 'p1' } })
    fireEvent.click(screen.getByTestId('bans-search-btn'))
    await waitFor(() => expect(screen.getByTestId('active-bans-table')).toBeInTheDocument())

    fireEvent.click(screen.getByTestId('lift-ban-b1'))
    await waitFor(() => expect(mockAdmin.liftBan).toHaveBeenCalledWith('b1'))
  })

  it('hides ban actions for player role', () => {
    render(<BansTab callerRole='player' />)
    expect(screen.queryByTestId('open-ban-dialog-btn')).not.toBeInTheDocument()
  })
})
