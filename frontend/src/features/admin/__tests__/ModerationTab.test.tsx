import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { PlayerReport } from '@/lib/schema-generated.zod'
import { ModerationTab } from '../components/ModerationTab'

const { mockToast, mockAdmin } = vi.hoisted(() => ({
  mockToast: { showError: vi.fn(), showWarning: vi.fn(), showInfo: vi.fn() },
  mockAdmin: {
    listReports: vi.fn(),
    reviewReport: vi.fn(),
  },
}))

vi.mock('@/ui/Toast', () => ({ useToast: () => mockToast }))
vi.mock('@/features/admin/api', () => ({ admin: mockAdmin }))

const REPORTS: PlayerReport[] = [
  {
    id: 'r1',
    reporter_id: 'p1',
    reported_id: 'p2',
    reason: 'Harassment',
    context: null,
    status: 'pending',
    created_at: '2025-01-10T00:00:00Z',
  },
  {
    id: 'r2',
    reporter_id: 'p3',
    reported_id: 'p4',
    reason: 'Cheating',
    context: null,
    status: 'reviewed',
    resolution: 'warn',
    reviewed_by: 'admin',
    reviewed_at: '2025-01-11T00:00:00Z',
    created_at: '2025-01-09T00:00:00Z',
  },
]

describe('ModerationTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAdmin.listReports.mockResolvedValue(REPORTS)
  })

  it('renders reports table', async () => {
    render(<ModerationTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('reports-table')).toBeInTheDocument())
    expect(screen.getByText('Harassment')).toBeInTheDocument()
  })

  it('shows empty state when no reports', async () => {
    mockAdmin.listReports.mockResolvedValue([])
    render(<ModerationTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('moderation-empty')).toBeInTheDocument())
  })

  it('opens detail view on row click', async () => {
    render(<ModerationTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('report-row-r1')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('report-row-r1'))
    expect(screen.getByTestId('report-detail')).toBeInTheDocument()
    expect(screen.getByText('Harassment')).toBeInTheDocument()
  })

  it('dismisses a report from detail view', async () => {
    mockAdmin.reviewReport.mockResolvedValue(undefined)
    render(<ModerationTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('report-row-r1')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('report-row-r1'))
    fireEvent.click(screen.getByTestId('dismiss-report-btn'))
    await waitFor(() => expect(mockAdmin.reviewReport).toHaveBeenCalledWith('r1', 'dismiss'))
  })

  it('warns on a report from detail view', async () => {
    mockAdmin.reviewReport.mockResolvedValue(undefined)
    render(<ModerationTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('report-row-r1')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('report-row-r1'))
    fireEvent.click(screen.getByTestId('warn-report-btn'))
    await waitFor(() => expect(mockAdmin.reviewReport).toHaveBeenCalledWith('r1', 'warn'))
  })

  it('shows ban button for manager/owner roles', async () => {
    const onBan = vi.fn()
    render(<ModerationTab callerRole='manager' onBanPlayer={onBan} />)
    await waitFor(() => expect(screen.getByTestId('report-row-r1')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('report-row-r1'))
    expect(screen.getByTestId('ban-from-report-btn')).toBeInTheDocument()
  })

  it('calls onBanPlayer when ban button clicked', async () => {
    const onBan = vi.fn()
    render(<ModerationTab callerRole='owner' onBanPlayer={onBan} />)
    await waitFor(() => expect(screen.getByTestId('report-row-r1')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('report-row-r1'))
    fireEvent.click(screen.getByTestId('ban-from-report-btn'))
    expect(onBan).toHaveBeenCalledWith('p2')
  })

  it('switches filter and refetches', async () => {
    render(<ModerationTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('reports-table')).toBeInTheDocument())
    fireEvent.click(screen.getByTestId('filter-reviewed'))
    await waitFor(() => expect(mockAdmin.listReports).toHaveBeenCalledWith('reviewed'))
  })
})
