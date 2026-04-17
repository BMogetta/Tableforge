import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ChatIcon, RoomToolbar, SettingsIcon } from '../RoomToolbar'

const baseItems = [
  { id: 'settings' as const, label: 'Settings', icon: SettingsIcon, visible: true },
  { id: 'chat' as const, label: 'Chat', icon: ChatIcon, visible: true },
]

describe('RoomToolbar', () => {
  it('renders visible toolbar buttons', () => {
    render(<RoomToolbar items={baseItems} activePopover={null} onToggle={vi.fn()} />)
    expect(screen.getByTitle('Settings')).toBeInTheDocument()
    expect(screen.getByTitle('Chat')).toBeInTheDocument()
  })

  it('hides items with visible=false', () => {
    const items = [
      { id: 'settings' as const, label: 'Settings', icon: SettingsIcon, visible: false },
      { id: 'chat' as const, label: 'Chat', icon: ChatIcon, visible: true },
    ]
    render(<RoomToolbar items={items} activePopover={null} onToggle={vi.fn()} />)
    expect(screen.queryByTitle('Settings')).not.toBeInTheDocument()
    expect(screen.getByTitle('Chat')).toBeInTheDocument()
  })

  it('calls onToggle with item id when clicked', () => {
    const onToggle = vi.fn()
    render(<RoomToolbar items={baseItems} activePopover={null} onToggle={onToggle} />)
    fireEvent.click(screen.getByTitle('Settings'))
    expect(onToggle).toHaveBeenCalledWith('settings')
  })

  it('applies active style to active popover button', () => {
    render(
      <RoomToolbar items={baseItems} activePopover='settings' onToggle={vi.fn()}>
        <div>popover content</div>
      </RoomToolbar>,
    )
    // Active button should have the active class
    const btn = screen.getByTitle('Settings')
    expect(btn.className).toContain('Active')
  })

  it('renders popover content when activePopover is set', () => {
    render(
      <RoomToolbar items={baseItems} activePopover='settings' onToggle={vi.fn()}>
        <div>Settings Panel</div>
      </RoomToolbar>,
    )
    expect(screen.getByText('Settings Panel')).toBeInTheDocument()
  })

  it('does not render popover when activePopover is null', () => {
    render(
      <RoomToolbar items={baseItems} activePopover={null} onToggle={vi.fn()}>
        <div>Settings Panel</div>
      </RoomToolbar>,
    )
    expect(screen.queryByText('Settings Panel')).not.toBeInTheDocument()
  })

  it('closes popover when backdrop is clicked', () => {
    const onToggle = vi.fn()
    const { container } = render(
      <RoomToolbar items={baseItems} activePopover='chat' onToggle={onToggle}>
        <div>Chat Content</div>
      </RoomToolbar>,
    )
    // The backdrop is a div with popoverBackdrop class
    const backdrop = container.querySelector('[class*="popoverBackdrop"]')
    expect(backdrop).toBeInTheDocument()
    fireEvent.click(backdrop!)
    expect(onToggle).toHaveBeenCalledWith('chat')
  })

  it('shows badge when provided', () => {
    const items = [{ id: 'chat' as const, label: 'Chat', icon: ChatIcon, visible: true, badge: 3 }]
    render(<RoomToolbar items={items} activePopover={null} onToggle={vi.fn()} />)
    expect(screen.getByText('3')).toBeInTheDocument()
  })

  it('shows 9+ for badge over 9', () => {
    const items = [{ id: 'chat' as const, label: 'Chat', icon: ChatIcon, visible: true, badge: 15 }]
    render(<RoomToolbar items={items} activePopover={null} onToggle={vi.fn()} />)
    expect(screen.getByText('9+')).toBeInTheDocument()
  })

  it('does not show badge when 0', () => {
    const items = [{ id: 'chat' as const, label: 'Chat', icon: ChatIcon, visible: true, badge: 0 }]
    render(<RoomToolbar items={items} activePopover={null} onToggle={vi.fn()} />)
    expect(screen.queryByText('0')).not.toBeInTheDocument()
  })
})
