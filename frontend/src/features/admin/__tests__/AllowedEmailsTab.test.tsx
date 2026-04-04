import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { AllowedEmailsTab } from '../components/AllowedEmailsTab'
import type { AllowedEmail } from '@/lib/schema-generated.zod'

const { mockToast, mockAdmin } = vi.hoisted(() => ({
  mockToast: { showError: vi.fn(), showWarning: vi.fn(), showInfo: vi.fn() },
  mockAdmin: {
    listEmails: vi.fn(),
    addEmail: vi.fn(),
    removeEmail: vi.fn(),
  },
}))

vi.mock('@/ui/Toast', () => ({ useToast: () => mockToast }))
vi.mock('@/features/admin/api', () => ({ admin: mockAdmin }))

const EMAIL_FIXTURES: AllowedEmail[] = [
  { email: 'alice@test.com', role: 'player', created_at: '2025-01-01T00:00:00Z' },
  {
    email: 'bob@test.com',
    role: 'manager',
    invited_by: 'admin',
    created_at: '2025-01-02T00:00:00Z',
  },
]

describe('AllowedEmailsTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAdmin.listEmails.mockResolvedValue(EMAIL_FIXTURES)
  })

  it('renders the email table after loading', async () => {
    render(<AllowedEmailsTab callerRole='owner' />)
    expect(screen.getByText('Loading...')).toBeInTheDocument()
    await waitFor(() => expect(screen.getByTestId('emails-table')).toBeInTheDocument())
    expect(screen.getByText('alice@test.com')).toBeInTheDocument()
    expect(screen.getByText('bob@test.com')).toBeInTheDocument()
  })

  it('shows empty state when no emails', async () => {
    mockAdmin.listEmails.mockResolvedValue([])
    render(<AllowedEmailsTab callerRole='owner' />)
    await waitFor(() => expect(screen.getByTestId('emails-empty')).toBeInTheDocument())
  })

  it('shows role select only for owner', async () => {
    const { unmount } = render(<AllowedEmailsTab callerRole='owner' />)
    await waitFor(() => expect(screen.getByTestId('emails-table')).toBeInTheDocument())
    expect(screen.getByTestId('email-role-select')).toBeInTheDocument()
    unmount()

    mockAdmin.listEmails.mockResolvedValue(EMAIL_FIXTURES)
    render(<AllowedEmailsTab callerRole='manager' />)
    await waitFor(() => expect(screen.getByTestId('emails-table')).toBeInTheDocument())
    expect(screen.queryByTestId('email-role-select')).not.toBeInTheDocument()
  })

  it('calls addEmail on form submit', async () => {
    const newEntry: AllowedEmail = {
      email: 'new@test.com',
      role: 'player',
      created_at: '2025-03-01T00:00:00Z',
    }
    mockAdmin.addEmail.mockResolvedValue(newEntry)
    render(<AllowedEmailsTab callerRole='owner' />)
    await waitFor(() => expect(screen.getByTestId('emails-table')).toBeInTheDocument())

    fireEvent.change(screen.getByTestId('email-input'), { target: { value: 'new@test.com' } })
    fireEvent.click(screen.getByTestId('add-email-btn'))

    await waitFor(() => expect(mockAdmin.addEmail).toHaveBeenCalledWith('new@test.com', 'player'))
    expect(screen.getByText('new@test.com')).toBeInTheDocument()
  })

  it('shows error on add failure', async () => {
    mockAdmin.addEmail.mockRejectedValue({ status: 400, message: 'Invalid email' })
    render(<AllowedEmailsTab callerRole='owner' />)
    await waitFor(() => expect(screen.getByTestId('emails-table')).toBeInTheDocument())

    fireEvent.change(screen.getByTestId('email-input'), { target: { value: 'bad' } })
    fireEvent.click(screen.getByTestId('add-email-btn'))

    await waitFor(() => expect(screen.getByText('Invalid email')).toBeInTheDocument())
  })
})
