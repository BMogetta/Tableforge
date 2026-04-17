import { render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { SystemStats } from '../api'
import { StatsTab } from '../components/StatsTab'

const { mockToast, mockAdmin } = vi.hoisted(() => ({
  mockToast: { showError: vi.fn(), showWarning: vi.fn(), showInfo: vi.fn() },
  mockAdmin: {
    getStats: vi.fn(),
  },
}))

vi.mock('@/ui/Toast', () => ({ useToast: () => mockToast }))
vi.mock('@/features/admin/api', () => ({ admin: mockAdmin }))

const STATS: SystemStats = {
  online_players: 12,
  active_rooms: 3,
  active_sessions: 2,
  total_players: 150,
  total_sessions_today: 47,
}

describe('StatsTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAdmin.getStats.mockResolvedValue(STATS)
  })

  it('renders all stat cards', async () => {
    render(<StatsTab />)
    await waitFor(() => expect(screen.getByTestId('stats-panel')).toBeInTheDocument())
    expect(screen.getByTestId('stat-online-players')).toHaveTextContent('12')
    expect(screen.getByTestId('stat-active-rooms')).toHaveTextContent('3')
    expect(screen.getByTestId('stat-active-sessions')).toHaveTextContent('2')
    expect(screen.getByTestId('stat-total-players')).toHaveTextContent('150')
    expect(screen.getByTestId('stat-sessions-today')).toHaveTextContent('47')
  })

  it('shows loading state initially', () => {
    mockAdmin.getStats.mockReturnValue(new Promise(() => {}))
    render(<StatsTab />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
  })

  it('shows empty state on null response', async () => {
    mockAdmin.getStats.mockResolvedValue(null)
    render(<StatsTab />)
    await waitFor(() => expect(screen.getByTestId('stats-empty')).toBeInTheDocument())
  })

  it('shows error toast on fetch failure', async () => {
    mockAdmin.getStats.mockRejectedValue(new Error('Network error'))
    render(<StatsTab />)
    await waitFor(() => expect(mockToast.showError).toHaveBeenCalled())
  })
})
