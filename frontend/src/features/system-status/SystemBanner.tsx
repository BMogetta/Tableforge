import { useFlag, useFlagsStatus } from '@unleash/proxy-client-react'
import { useTranslation } from 'react-i18next'
import { Flags } from '@/lib/flags'
import { testId } from '@/utils/testId'
import styles from './SystemBanner.module.css'

/**
 * Top-of-app status banner. Surfaces any feature flag whose state affects
 * the user's ability to use the app globally.
 *
 * Priority (only the highest-priority banner is rendered):
 *   1. maintenance-mode ON             → everything is paused
 *   2. ranked-matchmaking-enabled OFF  → ranked queue is paused
 *
 * Everything else (chat paused, achievements paused, hidden games) is
 * narrow enough in scope that an app-wide banner would be noise — those
 * features keep local hints (disabled inputs, paused rows, filtered lists).
 *
 * Never render more than one banner at a time: stacking makes the layout
 * jumpy and gives the user a shifting "what's wrong" signal instead of a
 * single prioritized headline.
 */
export function SystemBanner() {
  // flagsReady gates default-ON flags so we don't flash "ranked paused"
  // during the ~200ms cold-start window. Kill-switch flags (maintenance)
  // default OFF and are safe to read directly.
  const { flagsReady } = useFlagsStatus()
  const maintenance = useFlag(Flags.MaintenanceMode)
  const rankedFlag = useFlag(Flags.RankedMatchmakingEnabled)
  const rankedDown = flagsReady && !rankedFlag
  const { t } = useTranslation()

  if (maintenance) {
    return (
      <div
        role='status'
        aria-live='polite'
        className={styles.banner}
        data-variant='maintenance'
        {...testId('system-banner')}
      >
        <span className={styles.title}>{t('maintenance.title')}</span>
        <span className={styles.message}>{t('maintenance.message')}</span>
      </div>
    )
  }

  if (rankedDown) {
    return (
      <div
        role='status'
        aria-live='polite'
        className={styles.banner}
        data-variant='ranked-paused'
        {...testId('system-banner')}
      >
        <span className={styles.title}>{t('systemStatus.rankedPausedTitle')}</span>
        <span className={styles.message}>{t('systemStatus.rankedPausedMessage')}</span>
      </div>
    )
  }

  return null
}
