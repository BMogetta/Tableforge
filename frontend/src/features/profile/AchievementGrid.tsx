import { useQuery } from '@tanstack/react-query'
import { useTranslation } from 'react-i18next'
import { achievements as achievementsApi, type PlayerAchievement } from '@/lib/api'
import { keys } from '@/lib/queryClient'
import type { AchievementDefinition } from '@/lib/schema-generated.zod'
import { testId } from '@/utils/testId'
import { AchievementCard, type AchievementDef } from './AchievementCard'
import styles from './AchievementGrid.module.css'

/** Map the API payload onto the card's renderer-friendly shape. */
function toDef(d: AchievementDefinition): AchievementDef {
  return {
    key: d.key,
    nameKey: d.name_key,
    descriptionKey: d.description_key,
    type: d.type,
    gameId: d.game_id,
    tiers: d.tiers.map(t => ({
      threshold: t.threshold,
      nameKey: t.name_key,
      descriptionKey: t.description_key,
    })),
  }
}

interface Props {
  achievements: PlayerAchievement[]
  isLoading: boolean
}

export function AchievementGrid({ achievements: progress, isLoading }: Props) {
  const { t } = useTranslation()
  const { data, isLoading: defsLoading } = useQuery({
    queryKey: keys.achievementDefinitions(),
    // Registry is static per deploy; cache aggressively.
    staleTime: Number.POSITIVE_INFINITY,
    queryFn: achievementsApi.definitions,
  })

  if (isLoading || defsLoading || !data) {
    return <div className={styles.loading}>{t('profile.loadingAchievements')}</div>
  }

  const defs = data.definitions.map(toDef)
  const achievementMap = new Map(progress.map(a => [a.achievement_key, a]))

  return (
    <div className={styles.grid} {...testId('achievement-grid')}>
      {defs.map(def => (
        <AchievementCard key={def.key} def={def} achievement={achievementMap.get(def.key)} />
      ))}
    </div>
  )
}
