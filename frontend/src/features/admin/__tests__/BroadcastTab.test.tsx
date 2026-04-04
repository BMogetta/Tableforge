import { describe, it, expect, vi, beforeEach } from 'vitest'
import { render, screen, fireEvent, waitFor } from '@testing-library/react'
import { BroadcastTab } from '../components/BroadcastTab'

const { mockToast, mockAdmin } = vi.hoisted(() => ({
  mockToast: { showError: vi.fn(), showWarning: vi.fn(), showInfo: vi.fn() },
  mockAdmin: {
    sendBroadcast: vi.fn(),
  },
}))

vi.mock('@/ui/Toast', () => ({ useToast: () => mockToast }))
vi.mock('@/features/admin/api', () => ({ admin: mockAdmin }))

describe('BroadcastTab', () => {
  beforeEach(() => {
    vi.clearAllMocks()
    mockAdmin.sendBroadcast.mockResolvedValue(undefined)
  })

  it('renders the broadcast form', () => {
    render(<BroadcastTab />)
    expect(screen.getByTestId('broadcast-panel')).toBeInTheDocument()
    expect(screen.getByTestId('broadcast-message')).toBeInTheDocument()
    expect(screen.getByTestId('broadcast-type')).toBeInTheDocument()
    expect(screen.getByTestId('broadcast-send')).toBeInTheDocument()
  })

  it('disables send button when message is empty', () => {
    render(<BroadcastTab />)
    expect(screen.getByTestId('broadcast-send')).toBeDisabled()
  })

  it('enables send button when message is entered', () => {
    render(<BroadcastTab />)
    fireEvent.change(screen.getByTestId('broadcast-message'), {
      target: { value: 'Server restart in 5 minutes' },
    })
    expect(screen.getByTestId('broadcast-send')).not.toBeDisabled()
  })

  it('sends broadcast with correct params', async () => {
    render(<BroadcastTab />)

    fireEvent.change(screen.getByTestId('broadcast-message'), {
      target: { value: 'Maintenance window starting' },
    })
    fireEvent.change(screen.getByTestId('broadcast-type'), {
      target: { value: 'warning' },
    })
    fireEvent.click(screen.getByTestId('broadcast-send'))

    await waitFor(() =>
      expect(mockAdmin.sendBroadcast).toHaveBeenCalledWith('Maintenance window starting', 'warning'),
    )
  })

  it('clears message after successful send', async () => {
    render(<BroadcastTab />)

    fireEvent.change(screen.getByTestId('broadcast-message'), {
      target: { value: 'Hello everyone' },
    })
    fireEvent.click(screen.getByTestId('broadcast-send'))

    await waitFor(() => {
      expect(mockToast.showInfo).toHaveBeenCalledWith('Broadcast sent.')
    })
    expect(screen.getByTestId('broadcast-message')).toHaveValue('')
  })

  it('shows error toast on send failure', async () => {
    mockAdmin.sendBroadcast.mockRejectedValue(new Error('Send failed'))
    render(<BroadcastTab />)

    fireEvent.change(screen.getByTestId('broadcast-message'), {
      target: { value: 'Test' },
    })
    fireEvent.click(screen.getByTestId('broadcast-send'))

    await waitFor(() => expect(mockToast.showError).toHaveBeenCalled())
  })

  it('defaults to info type', () => {
    render(<BroadcastTab />)
    expect(screen.getByTestId('broadcast-type')).toHaveValue('info')
  })

  it('does not send whitespace-only message', () => {
    render(<BroadcastTab />)
    fireEvent.change(screen.getByTestId('broadcast-message'), {
      target: { value: '   ' },
    })
    expect(screen.getByTestId('broadcast-send')).toBeDisabled()
  })
})
