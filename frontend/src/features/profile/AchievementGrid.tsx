import { useQuery } from '@tanstack/react-query'
import { useFlag, useFlagsStatus } from '@unleash/proxy-client-react'
import { useTranslation } from 'react-i18next'
import { achievements as achievementsApi, type PlayerAchievement } from '@/lib/api'
import { Flags } from '@/lib/flags'
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
  // Default-on flag: cold start keeps tracking-paused notice hidden.
  const { flagsReady } = useFlagsStatus()
  const achievementsFlag = useFlag(Flags.AchievementsEnabled)
  const maintenanceOn = useFlag(Flags.MaintenanceMode)
  // Under maintenance, the global banner covers the paused reason — don't
  // stack a feature-specific notice on top.
  const trackingPaused = flagsReady && !achievementsFlag && !maintenanceOn

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
    <>
      {trackingPaused && (
        <div
          className={styles.pausedBanner}
          role="status"
          {...testId('achievements-paused')}
        >
          {t('achievements.paused')}
        </div>
      )}
      <div className={styles.grid} {...testId('achievement-grid')}>
        {defs.map(def => (
          <AchievementCard key={def.key} def={def} achievement={achievementMap.get(def.key)} />
        ))}
      </div>
    </>
  )
}
