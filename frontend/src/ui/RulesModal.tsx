import { useState } from 'react'
import { useFocusTrap } from '@/hooks/useFocusTrap'
import { GAME_RULES } from '@/games/registry'
import type { CardName } from '@/games/rootaccess/components/CardDisplay'
import styles from './RulesModal.module.css'
import { testId } from '@/utils/testId'

interface Props {
  /** Pre-select this game's tab when opening from in-game. */
  initialGameId?: string
  /** Cards currently in the player's hand (Root Access only). */
  handCards?: CardName[]
  onClose: () => void
}

export function RulesModal({ initialGameId, handCards, onClose }: Props) {
  const trapRef = useFocusTrap<HTMLDivElement>()
  const entries = GAME_RULES
  const initialIndex = initialGameId
    ? Math.max(0, entries.findIndex(e => e.id === initialGameId))
    : 0
  const [activeTab, setActiveTab] = useState(initialIndex)

  const active = entries[activeTab]

  return (
    <div
      className={styles.overlay}
      onClick={e => e.target === e.currentTarget && onClose()}
    >
      <div
        ref={trapRef}
        className={styles.modal}
        role="dialog"
        aria-modal="true"
        aria-labelledby="rules-modal-title"
        {...testId('rules-modal')}
      >
        <header className={styles.header}>
          <h2 className={styles.title} id="rules-modal-title">Game Rules</h2>
          <button className="btn btn-ghost btn-sm" onClick={onClose} aria-label="Close">
            ✕
          </button>
        </header>

        <nav className={styles.tabs} role="tablist">
          {entries.map((entry, i) => (
            <button
              key={entry.id}
              role="tab"
              aria-selected={i === activeTab}
              className={`${styles.tab} ${i === activeTab ? styles.tabActive : ''}`}
              onClick={() => setActiveTab(i)}
              {...testId(`rules-tab-${entry.id}`)}
            >
              {entry.label}
            </button>
          ))}
        </nav>

        <div className={styles.content} role="tabpanel">
          {active && <active.component handCards={handCards} />}
        </div>
      </div>
    </div>
  )
}
