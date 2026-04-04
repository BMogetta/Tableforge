import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { TicTacToeRules } from '../Rules'

describe('TicTacToeRules', () => {
  it('renders objective section', () => {
    render(<TicTacToeRules />)
    expect(screen.getByText('Objective')).toBeInTheDocument()
    expect(screen.getByText(/three of your marks/i)).toBeInTheDocument()
  })

  it('renders turn flow section', () => {
    render(<TicTacToeRules />)
    expect(screen.getByText('Turn Flow')).toBeInTheDocument()
    expect(screen.getByText(/alternate turns/i)).toBeInTheDocument()
  })

  it('renders win conditions section', () => {
    render(<TicTacToeRules />)
    expect(screen.getByText('Win Conditions')).toBeInTheDocument()
    expect(screen.getAllByText(/three in a row/i).length).toBeGreaterThanOrEqual(1)
    expect(screen.getByText(/all 9 cells are filled/i)).toBeInTheDocument()
  })
})
