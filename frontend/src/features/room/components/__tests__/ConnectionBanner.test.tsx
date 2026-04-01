import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { ConnectionBanner } from '../ConnectionBanner'

describe('ConnectionBanner', () => {
  it('renders nothing when connected', () => {
    const { container } = render(<ConnectionBanner status='connected' />)
    expect(container.innerHTML).toBe('')
  })

  it('shows connecting message', () => {
    render(<ConnectionBanner status='connecting' />)
    expect(screen.getByText('Connecting...')).toBeInTheDocument()
  })

  it('shows reconnecting message', () => {
    render(<ConnectionBanner status='reconnecting' />)
    expect(screen.getByText('Connection lost — reconnecting...')).toBeInTheDocument()
  })

  it('shows disconnected message', () => {
    render(<ConnectionBanner status='disconnected' />)
    expect(screen.getByText('Disconnected. Please refresh the page.')).toBeInTheDocument()
  })
})
