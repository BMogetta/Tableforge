import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { Notification } from '@/lib/api'
import { NotificationItem } from '../NotificationItem'

function makeNotification(overrides?: Partial<Notification>): Notification {
  return {
    id: 'n1',
    player_id: 'p1',
    type: 'friend_request',
    payload: { from_player_id: 'p2', from_username: 'alice' },
    created_at: new Date().toISOString(),
    ...overrides,
  }
}

describe('NotificationItem', () => {
  it('renders friend request description', () => {
    render(<NotificationItem notification={makeNotification()} />)
    expect(screen.getByText(/alice sent you a friend request/)).toBeInTheDocument()
  })

  it('renders room invitation description', () => {
    const n = makeNotification({
      type: 'room_invitation',
      payload: { from_player_id: 'p2', from_username: 'bob', room_id: 'r1', room_code: 'ABC' },
    })
    render(<NotificationItem notification={n} />)
    expect(screen.getByText(/bob invited you to a room/)).toBeInTheDocument()
  })

  it('renders ban description', () => {
    const n = makeNotification({
      type: 'ban_issued',
      payload: { reason: 'decline_threshold' },
    })
    render(<NotificationItem notification={n} />)
    expect(screen.getByText(/banned.*too many declines/)).toBeInTheDocument()
  })

  it('shows accept/decline buttons for actionable notifications', () => {
    render(
      <NotificationItem notification={makeNotification()} onAccept={vi.fn()} onDecline={vi.fn()} />,
    )
    expect(screen.getByTestId('accept-btn')).toBeInTheDocument()
    expect(screen.getByTestId('decline-btn')).toBeInTheDocument()
  })

  it('does not show action buttons for ban notifications', () => {
    const n = makeNotification({ type: 'ban_issued', payload: { reason: 'moderator' } })
    render(<NotificationItem notification={n} />)
    expect(screen.queryByTestId('accept-btn')).not.toBeInTheDocument()
  })

  it('shows action taken status instead of buttons', () => {
    const n = makeNotification({ action_taken: 'accepted' })
    render(<NotificationItem notification={n} />)
    expect(screen.getByTestId('action-taken')).toHaveTextContent('Accepted')
    expect(screen.queryByTestId('accept-btn')).not.toBeInTheDocument()
  })

  it('shows expired label when action has expired', () => {
    const n = makeNotification({
      action_expires_at: new Date(Date.now() - 86_400_000).toISOString(),
    })
    render(<NotificationItem notification={n} />)
    expect(screen.getByTestId('expired')).toHaveTextContent('Expired')
  })

  it('calls onAccept with notification id', () => {
    const onAccept = vi.fn()
    render(<NotificationItem notification={makeNotification()} onAccept={onAccept} />)
    fireEvent.click(screen.getByTestId('accept-btn'))
    expect(onAccept).toHaveBeenCalledWith('n1')
  })

  it('calls onDecline with notification id', () => {
    const onDecline = vi.fn()
    render(<NotificationItem notification={makeNotification()} onDecline={onDecline} />)
    fireEvent.click(screen.getByTestId('decline-btn'))
    expect(onDecline).toHaveBeenCalledWith('n1')
  })

  it('disables buttons when pending', () => {
    render(
      <NotificationItem
        notification={makeNotification()}
        onAccept={vi.fn()}
        onDecline={vi.fn()}
        pending={true}
      />,
    )
    expect(screen.getByTestId('accept-btn')).toBeDisabled()
    expect(screen.getByTestId('decline-btn')).toBeDisabled()
  })

  it('shows type label', () => {
    render(<NotificationItem notification={makeNotification()} />)
    expect(screen.getByText('Friend Request')).toBeInTheDocument()
  })

  it('shows relative time', () => {
    render(<NotificationItem notification={makeNotification()} />)
    expect(screen.getByText('just now')).toBeInTheDocument()
  })
})
