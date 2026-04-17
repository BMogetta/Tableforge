import { render, screen } from '@testing-library/react'
import { describe, expect, it } from 'vitest'
import type { PlayerAchievement } from '@/lib/api'
import { AchievementCard, type AchievementDef } from '../AchievementCard'
import { AchievementGrid } from '../AchievementGrid'
import '@/lib/i18n'

// All defs carry i18n keys; resolution happens inside AchievementCard via
// useTranslation. The component tests assert on *resolved* strings to exercise
// the key shape end-to-end rather than trusting raw keys.
const tieredDef: AchievementDef = {
  key: 'games_played',
  nameKey: 'achievements.games_played.name',
  type: 'tiered',
  gameId: '',
  tiers: [
    {
      threshold: 1,
      nameKey: 'achievements.games_played.tiers.1.name',
      descriptionKey: 'achievements.games_played.tiers.1.description',
    },
    {
      threshold: 10,
      nameKey: 'achievements.games_played.tiers.2.name',
      descriptionKey: 'achievements.games_played.tiers.2.description',
    },
    {
      threshold: 50,
      nameKey: 'achievements.games_played.tiers.3.name',
      descriptionKey: 'achievements.games_played.tiers.3.description',
    },
  ],
}

const flatDef: AchievementDef = {
  key: 'first_draw',
  nameKey: 'achievements.first_draw.name',
  descriptionKey: 'achievements.first_draw.description',
  type: 'flat',
  gameId: '',
  tiers: [
    {
      threshold: 1,
      nameKey: 'achievements.first_draw.tiers.1.name',
      descriptionKey: 'achievements.first_draw.tiers.1.description',
    },
  ],
}

const gameDef: AchievementDef = {
  key: 'ttt_perfect_game',
  nameKey: 'achievements.ttt_perfect_game.name',
  descriptionKey: 'achievements.ttt_perfect_game.description',
  type: 'flat',
  gameId: 'tictactoe',
  tiers: [
    {
      threshold: 1,
      nameKey: 'achievements.ttt_perfect_game.tiers.1.name',
      descriptionKey: 'achievements.ttt_perfect_game.tiers.1.description',
    },
  ],
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

  it('renders unlocked tier name resolved from i18n', () => {
    render(<AchievementCard def={tieredDef} achievement={makeAchievement('games_played', 1, 1)} />)
    expect(screen.getByText('Newcomer')).toBeInTheDocument()
  })

  it('interpolates {{threshold}} in tier descriptions', () => {
    render(<AchievementCard def={tieredDef} achievement={makeAchievement('games_played', 2, 20)} />)
    expect(screen.getByText('Play 10 games')).toBeInTheDocument()
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
    expect(screen.getByTestId('achievement-games_played')).toBeInTheDocument()
    expect(screen.getByTestId('achievement-games_won')).toBeInTheDocument()
    expect(screen.getByTestId('achievement-first_draw')).toBeInTheDocument()
  })

  it('matches achievements to definitions and resolves tier label', () => {
    const achievements = [makeAchievement('games_played', 2, 15)]
    render(<AchievementGrid achievements={achievements} isLoading={false} />)
    expect(screen.getByText('Regular')).toBeInTheDocument()
  })
})
