import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { TokenRow } from '../TokenRow'

interface TestToken {
  id: number
  owner: 'top' | 'bottom' | null
}

const tokens: TestToken[] = [
  { id: 1, owner: 'top' },
  { id: 2, owner: null },
  { id: 3, owner: 'bottom' },
]

function renderTestRow(overrides = {}) {
  const defaults = {
    tokens,
    renderToken: (t: TestToken) => <span>{t.id}</span>,
    getClaimState: (t: TestToken) => t.owner,
  }
  return render(<TokenRow<TestToken> {...defaults} {...overrides} />)
}

describe('TokenRow', () => {
  it('renders all tokens', () => {
    renderTestRow()
    expect(screen.getByText('1')).toBeInTheDocument()
    expect(screen.getByText('2')).toBeInTheDocument()
    expect(screen.getByText('3')).toBeInTheDocument()
  })

  it('renders correct number of token elements', () => {
    renderTestRow()
    expect(screen.getAllByTestId('token')).toHaveLength(3)
  })

  it('passes claim state to tokens', () => {
    renderTestRow()
    const tokenEls = screen.getAllByTestId('token')
    expect(tokenEls[0]).toHaveAttribute('aria-label', 'Token claimed by top')
    expect(tokenEls[1]).toHaveAttribute('aria-label', 'Unclaimed token')
    expect(tokenEls[2]).toHaveAttribute('aria-label', 'Token claimed by bottom')
  })

  it('renders empty row without errors', () => {
    renderTestRow({ tokens: [] })
    expect(screen.getByTestId('token-row')).toBeInTheDocument()
  })
})
