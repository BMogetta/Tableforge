import type { PlayerAchievement } from '@/lib/api'
import { testId } from '@/utils/testId'
import { AchievementCard, type AchievementDef } from './AchievementCard'
import styles from './AchievementGrid.module.css'

/** Achievement definitions mirroring the backend registry. */
const ACHIEVEMENTS: AchievementDef[] = [
  {
    key: 'games_played',
    name: 'Player',
    type: 'tiered',
    gameId: '',
    tiers: [
      { threshold: 1, name: 'Newcomer', description: 'Play your first game' },
      { threshold: 10, name: 'Regular', description: 'Play 10 games' },
      { threshold: 50, name: 'Dedicated', description: 'Play 50 games' },
      { threshold: 100, name: 'Veteran', description: 'Play 100 games' },
      { threshold: 500, name: 'Legend', description: 'Play 500 games' },
    ],
  },
  {
    key: 'games_won',
    name: 'Winner',
    type: 'tiered',
    gameId: '',
    tiers: [
      { threshold: 1, name: 'First Blood', description: 'Win your first game' },
      { threshold: 10, name: 'Skilled', description: 'Win 10 games' },
      { threshold: 50, name: 'Dominant', description: 'Win 50 games' },
      { threshold: 100, name: 'Champion', description: 'Win 100 games' },
    ],
  },
  {
    key: 'win_streak',
    name: 'On Fire',
    type: 'tiered',
    gameId: '',
    tiers: [
      { threshold: 3, name: 'Hot Streak', description: 'Win 3 games in a row' },
      { threshold: 5, name: 'Unstoppable', description: 'Win 5 games in a row' },
      { threshold: 10, name: 'Legendary', description: 'Win 10 games in a row' },
    ],
  },
  {
    key: 'first_draw',
    name: 'Stalemate',
    type: 'flat',
    gameId: '',
    tiers: [{ threshold: 1, name: 'Stalemate', description: 'Draw a game' }],
  },
  {
    key: 'ttt_perfect_game',
    name: 'Perfect Game',
    type: 'flat',
    gameId: 'tictactoe',
    tiers: [
      { threshold: 1, name: 'Perfect Game', description: 'Win in the minimum possible moves' },
    ],
  },
  {
    key: 'ttt_games_played',
    name: 'Tic-Tac-Toe Fan',
    type: 'tiered',
    gameId: 'tictactoe',
    tiers: [
      { threshold: 5, name: 'Beginner', description: 'Play 5 tic-tac-toe games' },
      { threshold: 25, name: 'Enthusiast', description: 'Play 25 tic-tac-toe games' },
      { threshold: 100, name: 'Addict', description: 'Play 100 tic-tac-toe games' },
    ],
  },
]

interface Props {
  achievements: PlayerAchievement[]
  isLoading: boolean
}

export function AchievementGrid({ achievements, isLoading }: Props) {
  if (isLoading) {
    return <div className={styles.loading}>Loading achievements...</div>
  }

  const achievementMap = new Map(achievements.map(a => [a.achievement_key, a]))

  return (
    <div className={styles.grid} {...testId('achievement-grid')}>
      {ACHIEVEMENTS.map(def => (
        <AchievementCard
          key={def.key}
          def={def}
          achievement={achievementMap.get(def.key)}
        />
      ))}
    </div>
  )
}
