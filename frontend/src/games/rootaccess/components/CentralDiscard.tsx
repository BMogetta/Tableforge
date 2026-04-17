import { AnimatePresence, motion } from 'motion/react'
import { useCallback, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, dealVariants, springTransition } from '@/ui/cards'
import { ModalOverlay } from '@/ui/ModalOverlay'
import { testId } from '@/utils/testId'
import type { CardName } from './CardDisplay'
import { CardFace } from './CardFace'
import styles from './CentralDiscard.module.css'

interface DiscardEntry {
  player_id: string
  card: CardName
}

interface Props {
  /** Global chronological order (preferred). */
  discardOrder?: DiscardEntry[]
  /** Per-player piles, used as fallback when discardOrder is absent. */
  discardPiles: Record<string, CardName[]>
  /** Resolve a player ID to a display username. */
  getUsername: (id: string) => string
  /** Exposed so other components can compute fly-out targets. */
  pileRef?: React.Ref<HTMLElement | null>
}

const STACK_OFFSETS = [
  { x: 0, y: 0 },
  { x: 3, y: -3 },
  { x: 6, y: -6 },
]

/** @package */
export function CentralDiscard({ discardOrder, discardPiles, getUsername, pileRef }: Props) {
  const { t } = useTranslation()
  const containerRef = useRef<HTMLDivElement | null>(null)
  const [showHistory, setShowHistory] = useState(false)

  const setRefs = useCallback(
    (node: HTMLDivElement | null) => {
      containerRef.current = node
      if (!pileRef) return
      if (typeof pileRef === 'function') {
        pileRef(node)
      } else {
        ;(pileRef as React.MutableRefObject<HTMLElement | null>).current = node
      }
    },
    [pileRef],
  )

  // Resolve top card: prefer global order, fall back to any non-empty pile.
  const total =
    discardOrder?.length ?? Object.values(discardPiles).reduce((sum, arr) => sum + arr.length, 0)

  const topCard: CardName | null = (() => {
    if (discardOrder && discardOrder.length > 0) {
      return discardOrder[discardOrder.length - 1].card
    }
    let picked: CardName | null = null
    let longest = 0
    for (const pile of Object.values(discardPiles)) {
      if (pile.length > longest) {
        longest = pile.length
        picked = pile[pile.length - 1] as CardName
      }
    }
    return picked
  })()

  const visibleLayers = Math.min(total, STACK_OFFSETS.length)
  const isEmpty = total === 0

  return (
    <div
      ref={setRefs}
      className={styles.wrapper}
      data-empty={isEmpty}
      {...testId('central-discard')}
    >
      <div
        role='button'
        tabIndex={isEmpty ? -1 : 0}
        aria-disabled={isEmpty}
        className={styles.pile}
        onClick={() => !isEmpty && setShowHistory(true)}
        onKeyDown={e => {
          if (isEmpty) return
          if (e.key === 'Enter' || e.key === ' ') {
            e.preventDefault()
            setShowHistory(true)
          }
        }}
        aria-label={isEmpty ? t('rootaccess.discardEmpty') : t('rootaccess.openDiscardHistory')}
      >
        {isEmpty ? (
          <div className={styles.placeholder}>
            <span className={styles.placeholderLabel}>{t('rootaccess.discards')}</span>
          </div>
        ) : (
          <AnimatePresence mode='popLayout'>
            {Array.from({ length: visibleLayers }, (_, i) => {
              const isTop = i === visibleLayers - 1
              const offset = STACK_OFFSETS[i]
              return (
                <motion.div
                  key={`layer-${i}-${isTop ? topCard : 'back'}`}
                  className={styles.layer}
                  style={{
                    transform: `translate(${offset.x}px, ${offset.y}px)`,
                    zIndex: i,
                  }}
                  variants={dealVariants}
                  initial='initial'
                  animate='animate'
                  exit='exit'
                  transition={springTransition}
                >
                  <Card
                    disabled={true}
                    front={
                      isTop && topCard ? (
                        <div className={styles.face}>
                          <CardFace card={topCard} />
                        </div>
                      ) : (
                        <div className={styles.face} />
                      )
                    }
                  />
                </motion.div>
              )
            })}
          </AnimatePresence>
        )}
      </div>
      {total > 0 && (
        <span className={styles.count} aria-label={`${total} cards discarded`}>
          {total}
        </span>
      )}

      {showHistory && total > 0 && (
        <ModalOverlay onClose={() => setShowHistory(false)}>
          <div
            className={styles.historyModal}
            role='dialog'
            aria-modal='true'
            aria-labelledby='discard-history-title'
            {...testId('central-discard-history')}
          >
            <header className={styles.historyHeader}>
              <h2 id='discard-history-title' className={styles.historyTitle}>
                {t('rootaccess.discardHistory')}
              </h2>
              <button
                type='button'
                className='btn btn-ghost btn-sm'
                onClick={() => setShowHistory(false)}
                aria-label={t('common.closeDialog')}
              >
                ✕
              </button>
            </header>
            <div className={styles.historyBody}>
              {Object.entries(discardPiles)
                .filter(([, cards]) => cards.length > 0)
                .map(([playerId, cards]) => (
                  <div key={playerId} className={styles.playerGroup}>
                    <span className={styles.playerName}>{getUsername(playerId)}</span>
                    <div className={styles.cardRow}>
                      {cards.map((card, i) => (
                        <div key={`${card}-${i}`} className={styles.historyCard}>
                          <CardFace card={card as CardName} />
                        </div>
                      ))}
                    </div>
                  </div>
                ))}
            </div>
          </div>
        </ModalOverlay>
      )}
    </div>
  )
}
