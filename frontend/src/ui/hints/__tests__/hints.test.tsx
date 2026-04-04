import { describe, it, expect, beforeEach } from 'vitest'
import { render, screen } from '@testing-library/react'
import userEvent from '@testing-library/user-event'
import { DimOverlay } from '../DimOverlay'
import { TooltipWrapper } from '../TooltipWrapper'
import { HighlightBorder } from '../HighlightBorder'
import { HintText } from '../HintText'

describe('DimOverlay', () => {
  it('applies dimOverlay class when dimmed is true', () => {
    const { container } = render(
      <DimOverlay dimmed={true}>
        <span>Content</span>
      </DimOverlay>,
    )
    expect(container.firstChild).toHaveClass(/dimOverlay/)
  })

  it('does not apply dimOverlay class when dimmed is false', () => {
    const { container } = render(
      <DimOverlay dimmed={false}>
        <span>Content</span>
      </DimOverlay>,
    )
    expect(container.firstChild).not.toHaveClass(/dimOverlay/)
  })

  it('renders children', () => {
    render(
      <DimOverlay dimmed={false}>
        <span>Child text</span>
      </DimOverlay>,
    )
    expect(screen.getByText('Child text')).toBeInTheDocument()
  })
})

describe('TooltipWrapper', () => {
  it('does not show tooltip by default', () => {
    render(
      <TooltipWrapper text="Tooltip text">
        <button>Hover me</button>
      </TooltipWrapper>,
    )
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })

  it('shows tooltip on mouse enter', async () => {
    const user = userEvent.setup()
    render(
      <TooltipWrapper text="Tooltip text">
        <button>Hover me</button>
      </TooltipWrapper>,
    )
    await user.hover(screen.getByText('Hover me'))
    expect(screen.getByRole('tooltip')).toHaveTextContent('Tooltip text')
  })

  it('hides tooltip on mouse leave', async () => {
    const user = userEvent.setup()
    render(
      <TooltipWrapper text="Tooltip text">
        <button>Hover me</button>
      </TooltipWrapper>,
    )
    await user.hover(screen.getByText('Hover me'))
    expect(screen.getByRole('tooltip')).toBeInTheDocument()
    await user.unhover(screen.getByText('Hover me'))
    expect(screen.queryByRole('tooltip')).not.toBeInTheDocument()
  })
})

describe('HighlightBorder', () => {
  it('applies highlightBorder class when highlighted is true', () => {
    const { container } = render(
      <HighlightBorder highlighted={true}>
        <span>Content</span>
      </HighlightBorder>,
    )
    expect(container.firstChild).toHaveClass(/highlightBorder/)
  })

  it('does not apply class when highlighted is false', () => {
    const { container } = render(
      <HighlightBorder highlighted={false}>
        <span>Content</span>
      </HighlightBorder>,
    )
    expect(container.firstChild).not.toHaveClass(/highlightBorder/)
  })
})

describe('HintText', () => {
  it('renders the text', () => {
    render(<HintText text="Must play ENCRYPTED_KEY" />)
    expect(screen.getByText('Must play ENCRYPTED_KEY')).toBeInTheDocument()
  })

  it('applies hintText class', () => {
    const { container } = render(<HintText text="hint" />)
    expect(container.firstChild).toHaveClass(/hintText/)
  })
})
