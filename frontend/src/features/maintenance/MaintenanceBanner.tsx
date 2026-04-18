import { useFlag } from '@unleash/proxy-client-react'
import { useTranslation } from 'react-i18next'
import { Flags } from '@/lib/flags'
import { testId } from '@/utils/testId'
import styles from './MaintenanceBanner.module.css'

/**
 * Full-width banner rendered at the very top of the app when the
 * `maintenance-mode` flag is ON. Cosmetic only — the real 503 responses
 * come from the maintenance middleware on the services. Without this the
 * user would just see "Failed to do X" errors with no explanation.
 */
export function MaintenanceBanner() {
  const enabled = useFlag(Flags.MaintenanceMode)
  const { t } = useTranslation()

  if (!enabled) return null

  return (
    <div
      role="status"
      aria-live="polite"
      className={styles.banner}
      {...testId('maintenance-banner')}
    >
      <span className={styles.title}>{t('maintenance.title')}</span>
      <span className={styles.message}>{t('maintenance.message')}</span>
    </div>
  )
}
