import { type CardName, CARD_META } from './CardDisplay'
import styles from './TargetPicker.module.css'

interface Player {
  id: string
  username: string
}

interface Props {
  /** All players in the game except the local player. */
  opponents: Player[]
  /** Players currently eliminated this round — shown but unselectable. */
  eliminatedIds: string[]
  /** Players currently protected by Handmaid — shown but unselectable. */
  protectedIds: string[]
  /** Currently selected target player ID. */
  selectedTarget: string | null
  /** Card being played — determines whether guess dropdown is shown. */
  cardBeingPlayed: CardName
  /** Currently selected Guard guess. */
  selectedGuess: CardName | null
  onSelectTarget: (playerId: string) => void
  onSelectGuess: (card: CardName) => void
}

const GUARD_GUESS_OPTIONS: CardName[] = [
  'spy',
  'priest',
  'baron',
  'handmaid',
  'prince',
  'chancellor',
  'king',
  'countess',
  'princess',
]

export default function TargetPicker({
  opponents,
  eliminatedIds,
  protectedIds,
  selectedTarget,
  cardBeingPlayed,
  selectedGuess,
  onSelectTarget,
  onSelectGuess,
}: Props) {
  const allUnavailable = opponents.every(
    p => eliminatedIds.includes(p.id) || protectedIds.includes(p.id),
  )

  return (
    <div className={styles.root}>
      <div className={styles.section}>
        <span className={styles.label}>Choose target</span>
        {allUnavailable && (
          <span className={styles.hint}>
            All opponents are protected or eliminated — no effect.
          </span>
        )}
        <div className={styles.targets}>
          {opponents.map(p => {
            const isEliminated = eliminatedIds.includes(p.id)
            const isProtected = protectedIds.includes(p.id)
            const unavailable = isEliminated || isProtected
            const isSelected = selectedTarget === p.id

            return (
              <button
                key={p.id}
                className={[
                  styles.targetBtn,
                  isSelected ? styles.selected : '',
                  unavailable ? styles.unavailable : '',
                ]
                  .filter(Boolean)
                  .join(' ')}
                onClick={() => !unavailable && onSelectTarget(p.id)}
                disabled={unavailable}
                aria-pressed={isSelected}
              >
                <span className={styles.targetName}>{p.username}</span>
                {isProtected && <span className={styles.tag}>Protected</span>}
                {isEliminated && <span className={styles.tag}>Eliminated</span>}
              </button>
            )
          })}
        </div>
      </div>

      {cardBeingPlayed === 'guard' && (
        <div className={styles.section}>
          <span className={styles.label}>Guess their card</span>
          <select
            className={styles.guessSelect}
            value={selectedGuess ?? ''}
            onChange={e => onSelectGuess(e.target.value as CardName)}
          >
            <option value='' disabled>
              Select a card…
            </option>
            {GUARD_GUESS_OPTIONS.map(card => (
              <option key={card} value={card}>
                {CARD_META[card].value} — {CARD_META[card].label}
              </option>
            ))}
          </select>
        </div>
      )}
    </div>
  )
}
