import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { Card } from '../Card'

describe('Card', () => {
  it('renders front face content', () => {
    render(<Card front={<span>Ace</span>} />)
    expect(screen.getByText('Ace')).toBeInTheDocument()
  })

  it('renders custom back face content', () => {
    render(<Card front={<span>Ace</span>} back={<span>Custom back</span>} />)
    expect(screen.getByText('Custom back')).toBeInTheDocument()
  })

  it('renders default back pattern when no back prop', () => {
    const { container } = render(<Card front={<span>Ace</span>} />)
    expect(container.querySelector('[class*="defaultBack"]')).toBeInTheDocument()
  })

  it('calls onClick when clicked', () => {
    const onClick = vi.fn()
    render(<Card front={<span>Ace</span>} onClick={onClick} />)
    fireEvent.click(screen.getByTestId('card'))
    expect(onClick).toHaveBeenCalledOnce()
  })

  it('calls onFlip with toggled value when clicked', () => {
    const onFlip = vi.fn()
    render(<Card front={<span>Ace</span>} faceDown onFlip={onFlip} />)
    fireEvent.click(screen.getByTestId('card'))
    expect(onFlip).toHaveBeenCalledWith(false)
  })

  it('calls onFlip(true) when face-up card is clicked', () => {
    const onFlip = vi.fn()
    render(<Card front={<span>Ace</span>} faceDown={false} onFlip={onFlip} />)
    fireEvent.click(screen.getByTestId('card'))
    expect(onFlip).toHaveBeenCalledWith(true)
  })

  it('does not fire callbacks when disabled', () => {
    const onClick = vi.fn()
    const onFlip = vi.fn()
    render(<Card front={<span>Ace</span>} disabled onClick={onClick} onFlip={onFlip} />)
    fireEvent.click(screen.getByTestId('card'))
    expect(onClick).not.toHaveBeenCalled()
    expect(onFlip).not.toHaveBeenCalled()
  })

  it('sets aria-label based on faceDown state', () => {
    const { rerender } = render(<Card front={<span>Ace</span>} faceDown />)
    expect(screen.getByTestId('card')).toHaveAttribute('aria-label', 'Face-down card')

    rerender(<Card front={<span>Ace</span>} faceDown={false} />)
    expect(screen.getByTestId('card')).toHaveAttribute('aria-label', 'Face-up card')
  })

  it('marks disabled via data attribute', () => {
    render(<Card front={<span>Ace</span>} disabled />)
    expect(screen.getByTestId('card')).toHaveAttribute('data-disabled', 'true')
  })
})
