import { useTranslation } from 'react-i18next'
import type { BotProfile } from '@/lib/schema-generated.zod'
import { ErrorMessage } from '@/ui/ErrorMessage'
import type { AppError } from '@/utils/errors'
import { testId } from '@/utils/testId'
import styles from '../Room.module.css'

interface BotSectionProps {
  profiles: BotProfile[]
  selectedProfile: string
  onSelectProfile: (name: string) => void
  onAdd: () => void
  adding: boolean
  error: AppError | null
}

export function BotSection({
  profiles,
  selectedProfile,
  onSelectProfile,
  onAdd,
  adding,
  error: botError,
}: BotSectionProps) {
  const { t } = useTranslation()
  return (
    <section className={styles.botSection}>
      <p className='label'>{t('room.addBot')}</p>
      <div className={styles.botRow}>
        <select
          {...testId('add-bot-select')}
          className={styles.botSelect}
          value={selectedProfile}
          disabled={adding}
          onChange={e => onSelectProfile(e.target.value)}
        >
          {profiles.map(p => (
            <option key={p.name} value={p.name}>
              {p.name.charAt(0).toUpperCase() + p.name.slice(1)}
            </option>
          ))}
        </select>
        <button
          type='button'
          {...testId('add-bot-btn')}
          className='btn btn-secondary'
          disabled={adding}
          onClick={onAdd}
        >
          {adding ? t('room.addingBot') : t('room.addBot')}
        </button>
      </div>
      <ErrorMessage error={botError} />
    </section>
  )
}
