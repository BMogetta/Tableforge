import { describe, it, expect } from 'vitest'
import { render, screen } from '@testing-library/react'
import { AchievementCard, type AchievementDef } from '../AchievementCard'
import { AchievementGrid } from '../AchievementGrid'
import type { PlayerAchievement } from '@/lib/api'

// ---------------------------------------------------------------------------
// AchievementCard
// ---------------------------------------------------------------------------

const tieredDef: AchievementDef = {
  key: 'games_played',
  name: 'Player',
  type: 'tiered',
  gameId: '',
  tiers: [
    { threshold: 1, name: 'Newcomer', description: 'Play your first game' },
    { threshold: 10, name: 'Regular', description: 'Play 10 games' },
    { threshold: 50, name: 'Dedicated', description: 'Play 50 games' },
  ],
}

const flatDef: AchievementDef = {
  key: 'first_draw',
  name: 'Stalemate',
  type: 'flat',
  gameId: '',
  tiers: [{ threshold: 1, name: 'Stalemate', description: 'Draw a game' }],
}

const gameDef: AchievementDef = {
  key: 'ttt_perfect_game',
  name: 'Perfect Game',
  type: 'flat',
  gameId: 'tictactoe',
  tiers: [{ threshold: 1, name: 'Perfect Game', description: 'Win in minimum moves' }],
}

function makeAchievement(key: string, tier: number, progress: number): PlayerAchievement {
  return {
    id: 'ach-1',
    player_id: 'p1',
    achievement_key: key,
    tier,
    progress,
    unlocked_at: new Date().toISOString(),
  }
}

describe('AchievementCard', () => {
  it('renders locked state when no achievement', () => {
    render(<AchievementCard def={tieredDef} />)
    expect(screen.getByTestId('achievement-games_played')).toBeInTheDocument()
    expect(screen.getByText('?')).toBeInTheDocument()
    expect(screen.getByText('Player')).toBeInTheDocument()
  })

  it('renders unlocked tier name', () => {
    render(<AchievementCard def={tieredDef} achievement={makeAchievement('games_played', 1, 1)} />)
    expect(screen.getByText('Newcomer')).toBeInTheDocument()
  })

  it('shows progress bar for tiered not at max', () => {
    render(<AchievementCard def={tieredDef} achievement={makeAchievement('games_played', 1, 5)} />)
    expect(screen.getByText('5 / 10')).toBeInTheDocument()
  })

  it('shows MAX label when at max tier', () => {
    render(<AchievementCard def={tieredDef} achievement={makeAchievement('games_played', 3, 50)} />)
    expect(screen.getByText('MAX')).toBeInTheDocument()
  })

  it('does not show progress bar for flat achievements', () => {
    render(<AchievementCard def={flatDef} achievement={makeAchievement('first_draw', 1, 1)} />)
    expect(screen.queryByText(/\//)).not.toBeInTheDocument()
  })

  it('shows game tag when gameId is set', () => {
    render(<AchievementCard def={gameDef} />)
    expect(screen.getByText('tictactoe')).toBeInTheDocument()
  })

  it('renders tier dots for tiered achievements', () => {
    const { container } = render(
      <AchievementCard def={tieredDef} achievement={makeAchievement('games_played', 2, 15)} />,
    )
    const dots = container.querySelectorAll('[class*="dot"]')
    expect(dots.length).toBeGreaterThanOrEqual(3)
  })
})

// ---------------------------------------------------------------------------
// AchievementGrid
// ---------------------------------------------------------------------------

describe('AchievementGrid', () => {
  it('renders loading state', () => {
    render(<AchievementGrid achievements={[]} isLoading={true} />)
    expect(screen.queryByTestId('achievement-grid')).not.toBeInTheDocument()
  })

  it('renders all achievement definitions', () => {
    render(<AchievementGrid achievements={[]} isLoading={false} />)
    expect(screen.getByTestId('achievement-grid')).toBeInTheDocument()
    // Should render all 6 achievements from registry
    expect(screen.getByTestId('achievement-games_played')).toBeInTheDocument()
    expect(screen.getByTestId('achievement-games_won')).toBeInTheDocument()
    expect(screen.getByTestId('achievement-first_draw')).toBeInTheDocument()
  })

  it('matches achievements to definitions', () => {
    const achievements = [makeAchievement('games_played', 2, 15)]
    render(<AchievementGrid achievements={achievements} isLoading={false} />)
    expect(screen.getByText('Regular')).toBeInTheDocument()
  })
})
