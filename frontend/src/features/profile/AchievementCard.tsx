import { useTranslation } from 'react-i18next'
import type { PlayerAchievement } from '@/lib/api'
import { testId } from '@/utils/testId'
import styles from './AchievementCard.module.css'

/**
 * Achievement definition mirroring the backend registry. String fields hold
 * positional i18n keys (achievements.{key}.name, ...tiers.{N}.description) —
 * resolution happens at render time so the same record serves both locales.
 */
export interface AchievementDef {
  key: string
  nameKey: string
  descriptionKey?: string
  type: 'flat' | 'tiered'
  gameId: string
  tiers: { threshold: number; nameKey: string; descriptionKey: string }[]
}

interface Props {
  def: AchievementDef
  achievement?: PlayerAchievement
}

export function AchievementCard({ def, achievement }: Props) {
  const { t } = useTranslation()
  const unlocked = achievement != null && achievement.tier > 0
  const tier = achievement?.tier ?? 0
  const progress = achievement?.progress ?? 0
  const maxTier = def.tiers.length
  const atMax = tier >= maxTier
  const currentTierDef = tier > 0 ? def.tiers[tier - 1] : undefined
  const nextTierDef = !atMax ? def.tiers[tier] : undefined
  const nextThreshold = nextTierDef?.threshold ?? 0

  const progressPct = nextThreshold > 0 ? Math.min((progress / nextThreshold) * 100, 100) : 100

  const displayName = unlocked && currentTierDef ? t(currentTierDef.nameKey) : t(def.nameKey)
  const displayDescription = (() => {
    if (unlocked && currentTierDef) {
      return t(currentTierDef.descriptionKey, { threshold: currentTierDef.threshold })
    }
    const firstTier = def.tiers[0]
    return t(firstTier.descriptionKey, { threshold: firstTier.threshold })
  })()

  return (
    <div
      className={`${styles.card} ${unlocked ? styles.unlocked : styles.locked}`}
      {...testId(`achievement-${def.key}`)}
    >
      <div className={styles.icon}>{unlocked ? tierBadge(tier) : '?'}</div>

      <div className={styles.info}>
        <div className={styles.name}>{displayName}</div>
        <div className={styles.description}>{displayDescription}</div>

        {def.gameId && <div className={styles.gameTag}>{def.gameId}</div>}
      </div>

      {def.type === 'tiered' && !atMax && (
        <div className={styles.progressArea}>
          <div className={styles.progressBar}>
            <div className={styles.progressFill} style={{ width: `${progressPct}%` }} />
          </div>
          <div className={styles.progressLabel}>
            {progress} / {nextThreshold}
          </div>
        </div>
      )}

      {def.type === 'tiered' && atMax && unlocked && <div className={styles.maxTier}>MAX</div>}

      {def.type === 'tiered' && (
        <div className={styles.tierDots}>
          {def.tiers.map((_, i) => (
            <span key={i} className={`${styles.dot} ${i < tier ? styles.dotFilled : ''}`} />
          ))}
        </div>
      )}
    </div>
  )
}

function tierBadge(tier: number): string {
  const badges = [
    '',
    '\u2605',
    '\u2605\u2605',
    '\u2605\u2605\u2605',
    '\u2605\u2605\u2605\u2605',
    '\u2605\u2605\u2605\u2605\u2605',
  ]
  return badges[Math.min(tier, badges.length - 1)]
}
