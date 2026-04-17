import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
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

function renderDropdown(
  overrides?: Partial<{
    isMuted: boolean
    onMute: () => void
    onUnmute: () => void
    onBlock: () => void
    onUnblock: () => void
  }>,
) {
  const props = {
    target,
    isMuted: false,
    onMute: vi.fn(),
    onUnmute: vi.fn(),
    onBlock: vi.fn(),
    onUnblock: vi.fn(),
    onAddFriend: vi.fn(),
    onSendDM: vi.fn(),
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

  it('calls onAddFriend when clicking Add Friend', () => {
    const { onAddFriend } = renderDropdown()
    fireEvent.click(screen.getByText(/Add Friend/))
    expect(onAddFriend).toHaveBeenCalledTimes(1)
  })

  it('calls onSendDM when clicking Send DM', () => {
    const { onSendDM } = renderDropdown()
    fireEvent.click(screen.getByText(/Send DM/))
    expect(onSendDM).toHaveBeenCalledTimes(1)
  })
})
