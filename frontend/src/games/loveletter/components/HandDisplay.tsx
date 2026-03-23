import { CardDisplay, type CardName } from './CardDisplay'
import styles from './HandDisplay.module.css'

interface Props {
  /** The local player's hand (1 or 2 cards). */
  cards: CardName[]
  /** The card currently selected to play. */
  selectedCard: CardName | null
  /** If true, card selection is not allowed (not your turn, game over). */
  disabled: boolean
  /** Called when the player clicks a card to select it. */
  onSelect: (card: CardName) => void
  /** Cards that cannot be played due to Countess rule — shown but dimmed. */
  blockedCards?: CardName[]
}

export function HandDisplay({ cards, selectedCard, disabled, onSelect, blockedCards = [] }: Props) {
  if (cards.length === 0) {
    return (
      <div className={styles.empty}>
        <span>No cards in hand</span>
      </div>
    )
  }

  return (
    <div className={styles.hand}>
      {cards.map((card, i) => {
        const isBlocked = blockedCards.includes(card)
        const isSelected = selectedCard === card

        return (
          <div key={`${card}-${i}`} className={styles.cardSlot}>
            <CardDisplay
              card={card}
              selected={isSelected}
              disabled={disabled || isBlocked}
              onClick={!disabled && !isBlocked ? () => onSelect(card) : undefined}
            />
            {isBlocked && <span className={styles.blockedLabel}>Must play Countess</span>}
          </div>
        )
      })}
    </div>
  )
}
