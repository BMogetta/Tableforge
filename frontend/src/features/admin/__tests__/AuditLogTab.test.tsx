import { fireEvent, render, screen, waitFor } from '@testing-library/react'
import { beforeEach, describe, expect, it, vi } from 'vitest'
import type { AuditLog } from '../api'
import { AuditLogTab } from '../components/AuditLogTab'

const { mockToast, mockAdmin } = vi.hoisted(() => ({
  mockToast: { showError: vi.fn(), showWarning: vi.fn(), showInfo: vi.fn() },
  mockAdmin: {
    listAuditLogs: vi.fn(),
  },
}))

vi.mock('@/ui/Toast', () => ({ useToast: () => mockToast }))
vi.mock('@/features/admin/api', () => ({ admin: mockAdmin }))

const LOGS: AuditLog[] = [
  {
    id: 'log1',
    actor_id: 'actor-aaa-bbb',
    action: 'ban_issued',
    target_type: 'player',
    target_id: 'target-ccc-ddd',
    created_at: '2025-01-15T10:00:00Z',
  },
  {
    id: 'log2',
    actor_id: 'actor-eee-fff',
    action: 'email_added',
    target_type: 'email',
    target_id: 'target-ggg-hhh',
    created_at: '2025-01-14T09:00:00Z',
  },
]

describe('AuditLogTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAdmin.listAuditLogs.mockResolvedValue(LOGS)
  })

  it('renders audit log table', async () => {
    render(<AuditLogTab />)
    await waitFor(() => expect(screen.getByTestId('audit-table')).toBeInTheDocument())
    expect(screen.getByTestId('audit-row-log1')).toBeInTheDocument()
    expect(screen.getByTestId('audit-row-log2')).toBeInTheDocument()
  })

  it('shows empty state when no logs', async () => {
    mockAdmin.listAuditLogs.mockResolvedValue([])
    render(<AuditLogTab />)
    await waitFor(() => expect(screen.getByTestId('audit-empty')).toBeInTheDocument())
  })

  it('filters by action', async () => {
    render(<AuditLogTab />)
    await waitFor(() => expect(screen.getByTestId('audit-table')).toBeInTheDocument())

    fireEvent.change(screen.getByTestId('audit-filter-action'), {
      target: { value: 'ban_issued' },
    })

    await waitFor(() =>
      expect(mockAdmin.listAuditLogs).toHaveBeenCalledWith(
        expect.objectContaining({ action: 'ban_issued', offset: 0 }),
      ),
    )
  })

  it('filters by target type', async () => {
    render(<AuditLogTab />)
    await waitFor(() => expect(screen.getByTestId('audit-table')).toBeInTheDocument())

    fireEvent.change(screen.getByTestId('audit-filter-target'), {
      target: { value: 'player' },
    })

    await waitFor(() =>
      expect(mockAdmin.listAuditLogs).toHaveBeenCalledWith(
        expect.objectContaining({ target_type: 'player', offset: 0 }),
      ),
    )
  })

  it('paginates with next/prev buttons', async () => {
    // First page: return full page to enable "next"
    const fullPage = Array.from({ length: 50 }, (_, i) => ({
      ...LOGS[0],
      id: `log-${i}`,
    }))
    mockAdmin.listAuditLogs.mockResolvedValue(fullPage)

    render(<AuditLogTab />)
    await waitFor(() => expect(screen.getByTestId('audit-table')).toBeInTheDocument())

    // Prev should be disabled on page 1
    expect(screen.getByTestId('audit-prev')).toBeDisabled()

    // Click next
    fireEvent.click(screen.getByTestId('audit-next'))
    await waitFor(() =>
      expect(mockAdmin.listAuditLogs).toHaveBeenCalledWith(expect.objectContaining({ offset: 50 })),
    )
  })

  it('shows error toast on fetch failure', async () => {
    mockAdmin.listAuditLogs.mockRejectedValue(new Error('Server error'))
    render(<AuditLogTab />)
    await waitFor(() => expect(mockToast.showError).toHaveBeenCalled())
  })
})
