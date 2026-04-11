import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { ErrorScreen } from '../ErrorScreen'
import { Splash } from '../Splash'

describe('Splash', () => {
  it('renders logo and connecting label', () => {
    render(<Splash />)
    expect(screen.getByText('RECESS')).toBeInTheDocument()
    expect(screen.getByText('CONNECTING...')).toBeInTheDocument()
  })
})

describe('ErrorScreen', () => {
  const defaults = {
    error: new Error('test error message'),
    onReset: vi.fn(),
  }

  it('renders error message', () => {
    render(<ErrorScreen {...defaults} />)
    expect(screen.getByText('test error message')).toBeInTheDocument()
  })

  it('renders heading', () => {
    render(<ErrorScreen {...defaults} />)
    expect(screen.getByText('SOMETHING WENT WRONG')).toBeInTheDocument()
  })

  it('calls onReset when confirm button is clicked', () => {
    const onReset = vi.fn()
    render(<ErrorScreen {...defaults} onReset={onReset} />)
    fireEvent.click(screen.getByText('Try Again'))
    expect(onReset).toHaveBeenCalledTimes(1)
  })

  it('renders back to lobby button', () => {
    render(<ErrorScreen {...defaults} />)
    expect(screen.getByText('Back to Lobby')).toBeInTheDocument()
  })
})
