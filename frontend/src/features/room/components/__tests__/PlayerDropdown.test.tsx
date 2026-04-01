import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { PlayerDropdown } from '../PlayerDropdown'

const target = {
  id: '1',
  username: 'alice',
  role: 'player' as const,
  is_bot: false,
  seat: 0,
  joined_at: '',
  created_at: '',
}

function renderDropdown(overrides?: Partial<{
  isMuted: boolean
  onMute: () => void
  onUnmute: () => void
  onBlock: () => void
  onUnblock: () => void
}>) {
  const props = {
    target,
    isMuted: false,
    onMute: vi.fn(),
    onUnmute: vi.fn(),
    onBlock: vi.fn(),
    onUnblock: vi.fn(),
    ...overrides,
  }
  render(<PlayerDropdown {...props} />)
  return props
}

describe('PlayerDropdown', () => {
  it('shows mute option when not muted', () => {
    renderDropdown()
    expect(screen.getByText(/Mute/)).toBeInTheDocument()
    expect(screen.queryByText(/Unmute/)).not.toBeInTheDocument()
  })

  it('shows unmute option when muted', () => {
    renderDropdown({ isMuted: true })
    expect(screen.getByText(/Unmute/)).toBeInTheDocument()
  })

  it('calls onMute when clicking mute', () => {
    const { onMute } = renderDropdown()
    fireEvent.click(screen.getByText(/Mute/))
    expect(onMute).toHaveBeenCalledTimes(1)
  })

  it('calls onBlock when clicking block', () => {
    const { onBlock } = renderDropdown()
    fireEvent.click(screen.getByText(/Block/))
    expect(onBlock).toHaveBeenCalledTimes(1)
  })

  it('calls onUnblock when clicking unblock', () => {
    const { onUnblock } = renderDropdown()
    fireEvent.click(screen.getByText(/Unblock/))
    expect(onUnblock).toHaveBeenCalledTimes(1)
  })

  it('has disabled friend and DM buttons', () => {
    renderDropdown()
    expect(screen.getByText(/Add Friend/)).toBeDisabled()
    expect(screen.getByText(/Send DM/)).toBeDisabled()
  })
})
