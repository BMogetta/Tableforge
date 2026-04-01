import type { BotProfile } from '@/lib/api'
import type { AppError } from '@/utils/errors'
import { ErrorMessage } from '@/ui/ErrorMessage'
import styles from '../Room.module.css'

interface BotSectionProps {
  profiles: BotProfile[]
  selectedProfile: string
  onSelectProfile: (name: string) => void
  onAdd: () => void
  adding: boolean
  error: AppError | null
}

export function BotSection({ profiles, selectedProfile, onSelectProfile, onAdd, adding, error: botError }: BotSectionProps) {
  return (
    <section className={styles.botSection}>
      <p className='label'>Add Bot</p>
      <div className={styles.botRow}>
        <select
          data-testid='add-bot-select'
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
          data-testid='add-bot-btn'
          className='btn btn-secondary'
          disabled={adding}
          onClick={onAdd}
        >
          {adding ? 'Adding...' : 'Add Bot'}
        </button>
      </div>
      <ErrorMessage error={botError} />
    </section>
  )
}
