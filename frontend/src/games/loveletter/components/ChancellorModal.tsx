import { useState } from 'react'
import { type CardName } from './CardDisplay'
import { CardDisplay } from './CardDisplay'
import styles from './ChancellorModal.module.css'

interface Props {
  /** The 3 cards the Chancellor player must choose from. */
  choices: CardName[]
  onConfirm: (keep: CardName, returnCards: [CardName, CardName]) => void
}

export function ChancellorModal({ choices, onConfirm }: Props) {
  const [kept, setKept] = useState<CardName | null>(null)
  // returnOrder holds the 2 cards to return, in the order they'll go to the
  // bottom of the deck. Index 0 = first at bottom, index 1 = second at bottom.
  const [returnOrder, setReturnOrder] = useState<CardName[]>([])

  const remaining = choices.filter(c => c !== kept)

  function handleKeep(card: CardName) {
    setKept(card)
    setReturnOrder([])
  }

  function handleReturn(card: CardName) {
    if (returnOrder.includes(card)) {
      // Deselect — remove from return order.
      setReturnOrder(returnOrder.filter(c => c !== card))
      return
    }
    if (returnOrder.length < 2) {
      setReturnOrder([...returnOrder, card])
    }
  }

  const canConfirm = kept !== null && returnOrder.length === 2

  function handleConfirm() {
    if (!canConfirm) return
    onConfirm(kept!, [returnOrder[0], returnOrder[1]] as [CardName, CardName])
  }

  return (
    <div className={styles.overlay}>
      <div className={styles.modal} role='dialog' aria-modal='true' aria-labelledby='chancellor-title'>
        <h2 className={styles.title} id='chancellor-title'>Chancellor</h2>
        <p className={styles.description}>
          Choose 1 card to keep. The other 2 will be placed at the bottom of the deck — click them
          in the order you want them stacked.
        </p>

        <div className={styles.section}>
          <span className={styles.label}>Your choices</span>
          <div className={styles.cards}>
            {choices.map((card, i) => (
              <div key={`${card}-${i}`} className={styles.cardSlot}>
                <CardDisplay
                  card={card}
                  selected={kept === card}
                  disabled={kept !== null && kept !== card && !remaining.includes(card)}
                  onClick={() => handleKeep(card)}
                />
                {kept === card && <span className={styles.tagKeep}>Keep</span>}
                {kept !== null && returnOrder.includes(card) && (
                  <span
                    className={styles.tagReturn}
                    data-testid={`return-order-${returnOrder.indexOf(card) + 1}`}
                  >
                    {returnOrder.indexOf(card) + 1}
                  </span>
                )}
              </div>
            ))}
          </div>
        </div>

        {kept !== null && (
          <div className={styles.section}>
            <span className={styles.label}>
              Return order — click to set deck order ({returnOrder.length}/2)
            </span>
            <div className={styles.cards}>
              {remaining.map((card, i) => {
                const pos = returnOrder.indexOf(card)
                const isSelected = pos !== -1
                return (
                  <div key={`${card}-${i}`} className={styles.cardSlot}>
                    <CardDisplay
                      card={card}
                      selected={isSelected}
                      onClick={() => handleReturn(card)}
                    />
                    {isSelected && (
                      <span className={styles.tagReturn} data-testid={`return-order-${pos + 1}`}>
                        {pos + 1}
                      </span>
                    )}
                  </div>
                )
              })}
            </div>
            <p className={styles.hint}>Card 1 goes to the bottom, card 2 goes on top of it.</p>
          </div>
        )}

        <button
          className={`btn btn-primary ${styles.confirmBtn}`}
          onClick={handleConfirm}
          disabled={!canConfirm}
        >
          Confirm
        </button>
      </div>
    </div>
  )
}
