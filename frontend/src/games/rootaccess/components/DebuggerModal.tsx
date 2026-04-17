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
  // Track selection by index into `choices`, not by card name. Choices may
  // contain duplicates (e.g. two identical cards); tracking by value would
  // conflate them and prevent independently selecting each copy.
  const [keptIdx, setKeptIdx] = useState<number | null>(null)
  // returnOrder holds indices of the 2 cards to return, in the order they'll
  // go to the bottom of the Repository. [0] = first, [1] = second.
  const [returnOrder, setReturnOrder] = useState<number[]>([])

  function handleKeep(idx: number) {
    setKeptIdx(idx)
    setReturnOrder([])
  }

  function handleReturn(idx: number) {
    if (returnOrder.includes(idx)) {
      setReturnOrder(returnOrder.filter(i => i !== idx))
      return
    }
    if (returnOrder.length < 2) {
      setReturnOrder([...returnOrder, idx])
    }
  }

  const canConfirm = keptIdx !== null && returnOrder.length === 2

  function handleConfirm() {
    if (!canConfirm) return
    onConfirm(choices[keptIdx!], [choices[returnOrder[0]], choices[returnOrder[1]]] as [
      CardName,
      CardName,
    ])
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
            {choices.map((card, i) => {
              const returnPos = returnOrder.indexOf(i)
              return (
                <div key={i} className={styles.cardSlot}>
                  <Card
                    front={
                      <div className={styles.face}>
                        <CardFace card={card} />
                      </div>
                    }
                    onClick={() => handleKeep(i)}
                  />
                  {keptIdx === i && <span className={styles.tagKeep}>{t('rootaccess.keep')}</span>}
                  {keptIdx !== null && returnPos !== -1 && (
                    <span className={styles.tagReturn} {...testId(`return-order-${returnPos + 1}`)}>
                      {returnPos + 1}
                    </span>
                  )}
                </div>
              )
            })}
          </div>
        </div>

        {keptIdx !== null && (
          <div className={styles.section}>
            <span className={styles.label}>
              {t('rootaccess.returnOrder', { count: `${returnOrder.length}/2` })}
            </span>
            <div className={styles.cards}>
              {choices.map((card, i) => {
                if (i === keptIdx) return null
                const pos = returnOrder.indexOf(i)
                return (
                  <div key={i} className={styles.cardSlot}>
                    <Card
                      front={
                        <div className={styles.face}>
                          <CardFace card={card} />
                        </div>
                      }
                      onClick={() => handleReturn(i)}
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
          type='button'
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
