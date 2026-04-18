import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { render, screen, waitFor } from '@testing-library/react'
import { describe, expect, it, vi } from 'vitest'
import type { PlayerAchievement } from '@/lib/api'
import { AchievementCard, type AchievementDef } from '../AchievementCard'
import { AchievementGrid } from '../AchievementGrid'
import '@/lib/i18n'

// Control achievements-enabled flag from individual tests.
const flagState = { flagsReady: true, achievementsEnabled: true }
vi.mock('@unleash/proxy-client-react', () => ({
  useFlagsStatus: () => ({ flagsReady: flagState.flagsReady, flagsError: null }),
  useFlag: (_name: string) => flagState.achievementsEnabled,
}))

// Static registry fetched by AchievementGrid. Mocked here so the component
// test exercises the mapping + rendering path without hitting the network.
vi.mock('@/lib/api', async () => {
  const actual = await vi.importActual<typeof import('@/lib/api')>('@/lib/api')
  return {
    ...actual,
    achievements: {
      definitions: () =>
        Promise.resolve({
          definitions: [
            {
              key: 'games_played',
              name_key: 'achievements.games_played.name',
              game_id: '',
              type: 'tiered' as const,
              tiers: [
                {
                  threshold: 1,
                  name_key: 'achievements.games_played.tiers.1.name',
                  description_key: 'achievements.games_played.tiers.1.description',
                },
                {
                  threshold: 10,
                  name_key: 'achievements.games_played.tiers.2.name',
                  description_key: 'achievements.games_played.tiers.2.description',
                },
              ],
            },
            {
              key: 'games_won',
              name_key: 'achievements.games_won.name',
              game_id: '',
              type: 'tiered' as const,
              tiers: [
                {
                  threshold: 1,
                  name_key: 'achievements.games_won.tiers.1.name',
                  description_key: 'achievements.games_won.tiers.1.description',
                },
              ],
            },
            {
              key: 'first_draw',
              name_key: 'achievements.first_draw.name',
              description_key: 'achievements.first_draw.description',
              game_id: '',
              type: 'flat' as const,
              tiers: [
                {
                  threshold: 1,
                  name_key: 'achievements.first_draw.tiers.1.name',
                  description_key: 'achievements.first_draw.tiers.1.description',
                },
              ],
            },
          ],
        }),
    },
  }
})

function renderGrid(achievements: PlayerAchievement[], isLoading: boolean) {
  const qc = new QueryClient({ defaultOptions: { queries: { retry: false } } })
  return render(
    <QueryClientProvider client={qc}>
      <AchievementGrid achievements={achievements} isLoading={isLoading} />
    </QueryClientProvider>,
  )
}

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
  it('renders loading state while the caller signals loading', () => {
    renderGrid([], true)
    expect(screen.queryByTestId('achievement-grid')).not.toBeInTheDocument()
  })

  it('renders every registry entry returned by the API', async () => {
    renderGrid([], false)
    await waitFor(() => expect(screen.getByTestId('achievement-grid')).toBeInTheDocument())
    expect(screen.getByTestId('achievement-games_played')).toBeInTheDocument()
    expect(screen.getByTestId('achievement-games_won')).toBeInTheDocument()
    expect(screen.getByTestId('achievement-first_draw')).toBeInTheDocument()
  })

  it('matches achievements to definitions and resolves tier label', async () => {
    renderGrid([makeAchievement('games_played', 2, 15)], false)
    await waitFor(() => expect(screen.getByText('Regular')).toBeInTheDocument())
  })

  it('shows a paused banner when achievements-enabled flag is OFF', async () => {
    flagState.flagsReady = true
    flagState.achievementsEnabled = false
    try {
      renderGrid([], false)
      await waitFor(() =>
        expect(screen.getByTestId('achievements-paused')).toBeInTheDocument(),
      )
      // Grid itself still renders — existing progress stays visible.
      expect(screen.getByTestId('achievement-grid')).toBeInTheDocument()
    } finally {
      flagState.achievementsEnabled = true
    }
  })

  it('hides the paused banner until flagsReady is true', async () => {
    flagState.flagsReady = false
    flagState.achievementsEnabled = false
    try {
      renderGrid([], false)
      await waitFor(() =>
        expect(screen.getByTestId('achievement-grid')).toBeInTheDocument(),
      )
      // Cold-start: avoid flashing a paused banner even if the flag happens
      // to evaluate OFF before the first fetch.
      expect(screen.queryByTestId('achievements-paused')).not.toBeInTheDocument()
    } finally {
      flagState.flagsReady = true
      flagState.achievementsEnabled = true
    }
  })
})
