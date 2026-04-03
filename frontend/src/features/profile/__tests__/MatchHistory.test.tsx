import { describe, it, expect, vi } from 'vitest'
import { render, screen, fireEvent } from '@testing-library/react'
import { MatchHistory } from '../components/MatchHistory'
import type { MatchHistoryEntry } from '@/lib/schema-generated.zod'

function makeMatch(overrides?: Partial<MatchHistoryEntry>): MatchHistoryEntry {
  return {
    id: 'r1',
    session_id: 's1',
    game_id: 'tictactoe',
    outcome: 'win',
    ended_by: 'win',
    duration_secs: 120,
    created_at: '2026-04-02T12:00:00Z',
    ...overrides,
  }
}

describe('MatchHistory', () => {
  it('renders match list', () => {
    const matches = [makeMatch(), makeMatch({ id: 'r2', outcome: 'loss' })]
    render(
      <MatchHistory
        matches={matches}
        total={2}
        page={0}
        pageSize={20}
        isLoading={false}
        onPageChange={vi.fn()}
        onViewReplay={vi.fn()}
      />,
    )
    expect(screen.getAllByRole('button')).toHaveLength(2)
  })

  it('shows outcome badges', () => {
    render(
      <MatchHistory
        matches={[makeMatch({ outcome: 'win' }), makeMatch({ id: 'r2', outcome: 'loss' })]}
        total={2}
        page={0}
        pageSize={20}
        isLoading={false}
        onPageChange={vi.fn()}
        onViewReplay={vi.fn()}
      />,
    )
    expect(screen.getByText('win')).toBeInTheDocument()
    expect(screen.getByText('loss')).toBeInTheDocument()
  })

  it('shows loading state', () => {
    render(
      <MatchHistory
        matches={[]}
        total={0}
        page={0}
        pageSize={20}
        isLoading={true}
        onPageChange={vi.fn()}
        onViewReplay={vi.fn()}
      />,
    )
    expect(screen.getByText('Loading matches...')).toBeInTheDocument()
  })

  it('shows empty state', () => {
    render(
      <MatchHistory
        matches={[]}
        total={0}
        page={0}
        pageSize={20}
        isLoading={false}
        onPageChange={vi.fn()}
        onViewReplay={vi.fn()}
      />,
    )
    expect(screen.getByText('No matches played yet.')).toBeInTheDocument()
  })

  it('calls onViewReplay when clicking a match', () => {
    const onViewReplay = vi.fn()
    render(
      <MatchHistory
        matches={[makeMatch({ session_id: 'session-123' })]}
        total={1}
        page={0}
        pageSize={20}
        isLoading={false}
        onPageChange={vi.fn()}
        onViewReplay={onViewReplay}
      />,
    )
    fireEvent.click(screen.getByText('tictactoe'))
    expect(onViewReplay).toHaveBeenCalledWith('session-123')
  })

  it('formats duration correctly', () => {
    render(
      <MatchHistory
        matches={[makeMatch({ duration_secs: 185 })]}
        total={1}
        page={0}
        pageSize={20}
        isLoading={false}
        onPageChange={vi.fn()}
        onViewReplay={vi.fn()}
      />,
    )
    expect(screen.getByText('3m 5s')).toBeInTheDocument()
  })

  it('shows pagination when more than one page', () => {
    render(
      <MatchHistory
        matches={[makeMatch()]}
        total={40}
        page={0}
        pageSize={20}
        isLoading={false}
        onPageChange={vi.fn()}
        onViewReplay={vi.fn()}
      />,
    )
    expect(screen.getByText('1 / 2')).toBeInTheDocument()
    expect(screen.getByText('← Prev')).toBeDisabled()
    expect(screen.getByText('Next →')).not.toBeDisabled()
  })

  it('calls onPageChange when clicking next', () => {
    const onPageChange = vi.fn()
    render(
      <MatchHistory
        matches={[makeMatch()]}
        total={40}
        page={0}
        pageSize={20}
        isLoading={false}
        onPageChange={onPageChange}
        onViewReplay={vi.fn()}
      />,
    )
    fireEvent.click(screen.getByText('Next →'))
    expect(onPageChange).toHaveBeenCalledWith(1)
  })

  it('hides pagination when single page', () => {
    render(
      <MatchHistory
        matches={[makeMatch()]}
        total={1}
        page={0}
        pageSize={20}
        isLoading={false}
        onPageChange={vi.fn()}
        onViewReplay={vi.fn()}
      />,
    )
    expect(screen.queryByText('← Prev')).not.toBeInTheDocument()
  })
})
