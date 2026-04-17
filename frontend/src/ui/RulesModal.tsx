import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { GAME_RULES } from '@/games/registry'
import { useFocusTrap } from '@/hooks/useFocusTrap'
import { testId } from '@/utils/testId'
import { ModalOverlay } from './ModalOverlay'
import styles from './RulesModal.module.css'

interface Props {
  /** Pre-select this game's tab when opening from in-game. */
  initialGameId?: string
  /**
   * Opaque per-game context forwarded to the active rules component.
   * Current use: Root Access consumes the local player's hand as string IDs.
   * Typed as string[] here so the modal stays game-agnostic; each game's
   * rules component narrows to its own card-name union internally.
   */
  handCards?: string[]
  onClose: () => void
}

export function RulesModal({ initialGameId, handCards, onClose }: Props) {
  const { t } = useTranslation()
  const trapRef = useFocusTrap<HTMLDivElement>()
  const entries = GAME_RULES
  const initialIndex = initialGameId
    ? Math.max(
        0,
        entries.findIndex(e => e.id === initialGameId),
      )
    : 0
  const [activeTab, setActiveTab] = useState(initialIndex)

  const active = entries[activeTab]

  return (
    <ModalOverlay onClose={onClose}>
      <div
        ref={trapRef}
        className={styles.modal}
        role='dialog'
        aria-modal='true'
        aria-labelledby='rules-modal-title'
        {...testId('rules-modal')}
      >
        <header className={styles.header}>
          <h2 className={styles.title} id='rules-modal-title'>
            {t('rules.title')}
          </h2>
          <button
            type='button'
            className='btn btn-ghost btn-sm'
            onClick={onClose}
            aria-label={t('common.close')}
          >
            ✕
          </button>
        </header>

        <div className={styles.tabs} role='tablist'>
          {entries.map((entry, i) => (
            <button
              type='button'
              key={entry.id}
              role='tab'
              aria-selected={i === activeTab}
              className={`${styles.tab} ${i === activeTab ? styles.tabActive : ''}`}
              onClick={() => setActiveTab(i)}
              {...testId(`rules-tab-${entry.id}`)}
            >
              {entry.label}
            </button>
          ))}
        </div>

        <div className={styles.content} role='tabpanel'>
          {active && <active.component handCards={handCards} />}
        </div>
      </div>
    </ModalOverlay>
  )
}
