import { fireEvent, render, screen } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import { BotSection } from '../BotSection'

const profiles = [
  {
    name: 'easy',
    iterations: 100,
    determinizations: 5,
    exploration_c: 1.4,
    aggressiveness: 0.3,
    risk_aversion: 0.5,
  },
  {
    name: 'medium',
    iterations: 500,
    determinizations: 10,
    exploration_c: 1.4,
    aggressiveness: 0.5,
    risk_aversion: 0.5,
  },
  {
    name: 'hard',
    iterations: 2000,
    determinizations: 20,
    exploration_c: 1.4,
    aggressiveness: 0.7,
    risk_aversion: 0.3,
  },
]

const base = {
  profiles,
  selectedProfile: 'medium',
  onSelectProfile: vi.fn(),
  onAdd: vi.fn(),
  adding: false,
  error: null,
}

describe('BotSection', () => {
  it('renders profile options', () => {
    render(<BotSection {...base} />)
    const select = screen.getByTestId('add-bot-select')
    expect(select).toBeInTheDocument()
    expect(select.querySelectorAll('option')).toHaveLength(3)
  })

  it('capitalizes profile names', () => {
    render(<BotSection {...base} />)
    const options = screen.getByTestId('add-bot-select').querySelectorAll('option')
    expect(options[0]).toHaveTextContent('Easy')
    expect(options[1]).toHaveTextContent('Medium')
    expect(options[2]).toHaveTextContent('Hard')
  })

  it('calls onAdd when clicking add', () => {
    const onAdd = vi.fn()
    render(<BotSection {...base} onAdd={onAdd} />)
    fireEvent.click(screen.getByTestId('add-bot-btn'))
    expect(onAdd).toHaveBeenCalledTimes(1)
  })

  it('shows Adding... when adding', () => {
    render(<BotSection {...base} adding={true} />)
    expect(screen.getByTestId('add-bot-btn')).toHaveTextContent('Adding...')
    expect(screen.getByTestId('add-bot-btn')).toBeDisabled()
  })

  it('calls onSelectProfile when changing selection', () => {
    const onSelectProfile = vi.fn()
    render(<BotSection {...base} onSelectProfile={onSelectProfile} />)
    fireEvent.change(screen.getByTestId('add-bot-select'), { target: { value: 'hard' } })
    expect(onSelectProfile).toHaveBeenCalledWith('hard')
  })
})
