import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { FriendItem } from '../FriendItem'
import { PendingRequestItem } from '../PendingRequestItem'

// ---------------------------------------------------------------------------
// FriendItem
// ---------------------------------------------------------------------------

describe('FriendItem', () => {
  const defaults = {
    friendId: 'f1',
    username: 'alice',
    online: true,
    onDM: vi.fn(),
    onRemove: vi.fn(),
    onBlock: vi.fn(),
    removePending: false,
    blockPending: false,
  }

  it('renders username and presence', () => {
    render(<FriendItem {...defaults} />)
    expect(screen.getByTestId('friend-f1')).toBeInTheDocument()
    expect(screen.getByText('alice')).toBeInTheDocument()
  })

  it('calls onDM when DM button clicked', () => {
    const onDM = vi.fn()
    render(<FriendItem {...defaults} onDM={onDM} />)
    fireEvent.click(screen.getByTestId('dm-btn'))
    expect(onDM).toHaveBeenCalledWith('f1')
  })

  it('calls onRemove when remove button clicked', () => {
    const onRemove = vi.fn()
    render(<FriendItem {...defaults} onRemove={onRemove} />)
    fireEvent.click(screen.getByTestId('remove-btn'))
    expect(onRemove).toHaveBeenCalledWith('f1')
  })

  it('calls onBlock with username and avatar', () => {
    const onBlock = vi.fn()
    render(<FriendItem {...defaults} onBlock={onBlock} avatarUrl="https://example.com/a.png" />)
    fireEvent.click(screen.getByTestId('block-btn'))
    expect(onBlock).toHaveBeenCalledWith('f1', 'alice', 'https://example.com/a.png')
  })

  it('disables remove button when removePending', () => {
    render(<FriendItem {...defaults} removePending={true} />)
    expect(screen.getByTestId('remove-btn')).toBeDisabled()
    expect(screen.getByTestId('remove-btn')).toHaveTextContent('...')
  })

  it('disables block button when blockPending', () => {
    render(<FriendItem {...defaults} blockPending={true} />)
    expect(screen.getByTestId('block-btn')).toBeDisabled()
    expect(screen.getByTestId('block-btn')).toHaveTextContent('...')
  })

  it('renders avatar when provided', () => {
    const { container } = render(<FriendItem {...defaults} avatarUrl="https://example.com/a.png" />)
    const img = container.querySelector('img')
    expect(img).toBeInTheDocument()
    expect(img?.src).toBe('https://example.com/a.png')
  })

  it('shows online presence dot', () => {
    const { container } = render(<FriendItem {...defaults} online={true} />)
    const dot = container.querySelector('[data-online="true"]')
    expect(dot).toBeInTheDocument()
  })

  it('shows offline presence dot', () => {
    const { container } = render(<FriendItem {...defaults} online={false} />)
    const dot = container.querySelector('[data-online="false"]')
    expect(dot).toBeInTheDocument()
  })
})

// ---------------------------------------------------------------------------
// PendingRequestItem
// ---------------------------------------------------------------------------

describe('PendingRequestItem', () => {
  const defaults = {
    requesterId: 'r1',
    username: 'bob',
    onAccept: vi.fn(),
    onDecline: vi.fn(),
    pending: false,
  }

  it('renders username', () => {
    render(<PendingRequestItem {...defaults} />)
    expect(screen.getByTestId('pending-r1')).toBeInTheDocument()
    expect(screen.getByText('bob')).toBeInTheDocument()
  })

  it('calls onAccept when accept button clicked', () => {
    const onAccept = vi.fn()
    render(<PendingRequestItem {...defaults} onAccept={onAccept} />)
    fireEvent.click(screen.getByTestId('accept-btn'))
    expect(onAccept).toHaveBeenCalledWith('r1')
  })

  it('calls onDecline when decline button clicked', () => {
    const onDecline = vi.fn()
    render(<PendingRequestItem {...defaults} onDecline={onDecline} />)
    fireEvent.click(screen.getByTestId('decline-btn'))
    expect(onDecline).toHaveBeenCalledWith('r1')
  })

  it('disables both buttons when pending', () => {
    render(<PendingRequestItem {...defaults} pending={true} />)
    expect(screen.getByTestId('accept-btn')).toBeDisabled()
    expect(screen.getByTestId('decline-btn')).toBeDisabled()
  })

  it('renders avatar when provided', () => {
    const { container } = render(
      <PendingRequestItem {...defaults} avatarUrl="https://example.com/b.png" />,
    )
    const img = container.querySelector('img')
    expect(img).toBeInTheDocument()
  })
})
