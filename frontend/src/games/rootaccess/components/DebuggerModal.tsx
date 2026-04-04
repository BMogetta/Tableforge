import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useFocusTrap } from '@/hooks/useFocusTrap'
import { Card } from '@/ui/cards'
import { testId } from '@/utils/testId'
import type { CardName } from './CardDisplay'
import { CardFace } from './CardFace'
import styles from './DebuggerModal.module.css'

interface Props {
  /** The 3 cards the Debugger player must choose from. */
  choices: CardName[]
  onConfirm: (keep: CardName, returnCards: [CardName, CardName]) => void
}

/** @package */
export function DebuggerModal({ choices, onConfirm }: Props) {
  const { t } = useTranslation()
  const trapRef = useFocusTrap<HTMLDivElement>()
  const [kept, setKept] = useState<CardName | null>(null)
  // returnOrder holds the 2 cards to return, in the order they'll go to the
  // bottom of the Repository. Index 0 = first at bottom, index 1 = second at bottom.
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
      <div
        ref={trapRef}
        className={styles.modal}
        role='dialog'
        aria-modal='true'
        aria-labelledby='debugger-title'
      >
        <h2 className={styles.title} id='debugger-title'>
          {t('rootaccess.debugger')}
        </h2>
        <p className={styles.description}>{t('rootaccess.debuggerDesc')}</p>

        <div className={styles.section}>
          <span className={styles.label}>{t('rootaccess.yourChoices')}</span>
          <div className={styles.cards}>
            {choices.map((card, i) => (
              <div key={`${card}-${i}`} className={styles.cardSlot}>
                <Card
                  front={
                    <div className={styles.face}>
                      <CardFace card={card} />
                    </div>
                  }
                  onClick={() => handleKeep(card)}
                  disabled={kept !== null && kept !== card && !remaining.includes(card)}
                />
                {kept === card && <span className={styles.tagKeep}>{t('rootaccess.keep')}</span>}
                {kept !== null && returnOrder.includes(card) && (
                  <span
                    className={styles.tagReturn}
                    {...testId(`return-order-${returnOrder.indexOf(card) + 1}`)}
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
              {t('rootaccess.returnOrder', { count: `${returnOrder.length}/2` })}
            </span>
            <div className={styles.cards}>
              {remaining.map((card, i) => {
                const pos = returnOrder.indexOf(card)
                return (
                  <div key={`${card}-${i}`} className={styles.cardSlot}>
                    <Card
                      front={
                        <div className={styles.face}>
                          <CardFace card={card} />
                        </div>
                      }
                      onClick={() => handleReturn(card)}
                    />
                    {pos !== -1 && (
                      <span className={styles.tagReturn} {...testId(`return-order-${pos + 1}`)}>
                        {pos + 1}
                      </span>
                    )}
                  </div>
                )
              })}
            </div>
            <p className={styles.hint}>{t('rootaccess.returnOrderHint')}</p>
          </div>
        )}

        <button
          className={`btn btn-primary ${styles.confirmBtn}`}
          onClick={handleConfirm}
          disabled={!canConfirm}
        >
          {t('common.confirm')}
        </button>
      </div>
    </div>
  )
}
