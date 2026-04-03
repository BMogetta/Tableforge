import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Token } from '../Token'

describe('Token', () => {
  it('renders children content', () => {
    render(<Token>5</Token>)
    expect(screen.getByText('5')).toBeInTheDocument()
  })

  it('defaults to unclaimed aria-label', () => {
    render(<Token>1</Token>)
    expect(screen.getByTestId('token')).toHaveAttribute('aria-label', 'Unclaimed token')
  })

  it('shows claimed aria-label when claimedBy is set', () => {
    render(<Token claimedBy="top">1</Token>)
    expect(screen.getByTestId('token')).toHaveAttribute('aria-label', 'Token claimed by top')
  })

  it('calls onClick when clicked', () => {
    const onClick = vi.fn()
    render(<Token onClick={onClick}>1</Token>)
    fireEvent.click(screen.getByTestId('token'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('does not call onClick when disabled', () => {
    const onClick = vi.fn()
    render(<Token disabled onClick={onClick}>1</Token>)
    fireEvent.click(screen.getByTestId('token'))
    expect(onClick).not.toHaveBeenCalled()
  })

  it('marks disabled via data attribute', () => {
    render(<Token disabled>1</Token>)
    expect(screen.getByTestId('token')).toHaveAttribute('data-disabled', 'true')
  })
})
