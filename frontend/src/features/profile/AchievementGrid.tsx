import { useTranslation } from 'react-i18next'
import type { PlayerAchievement } from '@/lib/api'
import { testId } from '@/utils/testId'
import { AchievementCard, type AchievementDef } from './AchievementCard'
import styles from './AchievementGrid.module.css'

// ---------------------------------------------------------------------------
// Positional i18n key helpers — must stay in sync with the backend registry
// at services/user-service/internal/achievements/registry.go. Once Phase C
// lands the definitions themselves will ship from the backend and this file
// shrinks to a pure renderer; the key shape remains the contract.
// ---------------------------------------------------------------------------

const nameKey = (key: string) => `achievements.${key}.name`
const descriptionKey = (key: string) => `achievements.${key}.description`
const tierNameKey = (key: string, tier: number) => `achievements.${key}.tiers.${tier}.name`
const tierDescriptionKey = (key: string, tier: number) =>
  `achievements.${key}.tiers.${tier}.description`

/** Build a tiered definition with keys auto-derived from (key, index). */
function tieredDef(key: string, gameId: string, thresholds: number[]): AchievementDef {
  return {
    key,
    nameKey: nameKey(key),
    type: 'tiered',
    gameId,
    tiers: thresholds.map((threshold, i) => ({
      threshold,
      nameKey: tierNameKey(key, i + 1),
      descriptionKey: tierDescriptionKey(key, i + 1),
    })),
  }
}

/** Build a flat definition (single tier, no progression). */
function flatDef(key: string, gameId: string, threshold: number): AchievementDef {
  return {
    key,
    nameKey: nameKey(key),
    descriptionKey: descriptionKey(key),
    type: 'flat',
    gameId,
    tiers: [
      {
        threshold,
        nameKey: tierNameKey(key, 1),
        descriptionKey: tierDescriptionKey(key, 1),
      },
    ],
  }
}

const ACHIEVEMENTS: AchievementDef[] = [
  tieredDef('games_played', '', [1, 10, 50, 100, 500]),
  tieredDef('games_won', '', [1, 10, 50, 100]),
  tieredDef('win_streak', '', [3, 5, 10]),
  flatDef('first_draw', '', 1),
  flatDef('ttt_perfect_game', 'tictactoe', 1),
  tieredDef('ttt_games_played', 'tictactoe', [5, 25, 100]),
]

interface Props {
  achievements: PlayerAchievement[]
  isLoading: boolean
}

export function AchievementGrid({ achievements, isLoading }: Props) {
  const { t } = useTranslation()

  if (isLoading) {
    return <div className={styles.loading}>{t('profile.loadingAchievements')}</div>
  }

  const achievementMap = new Map(achievements.map(a => [a.achievement_key, a]))

  return (
    <div className={styles.grid} {...testId('achievement-grid')}>
      {ACHIEVEMENTS.map(def => (
        <AchievementCard key={def.key} def={def} achievement={achievementMap.get(def.key)} />
      ))}
    </div>
  )
}
