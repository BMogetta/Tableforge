import { AnimatePresence, motion } from 'motion/react'
import type { RefObject } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, dealVariants, springTransition } from '@/ui/cards'
import { HintText, TooltipWrapper, useHintsEnabled } from '@/ui/hints'
import { CARD_META, type CardName } from './CardDisplay'
import { CardFace } from './CardFace'
import styles from './HandDisplay.module.css'

interface Props {
  cards: CardName[]
  selectedCard: CardName | null
  disabled: boolean
  onSelect: (card: CardName) => void
  blockedCards?: CardName[]
  /** Ref to the zone where played cards should fly toward. */
  targetRef?: RefObject<HTMLElement | null>
}

const SPREAD_ANGLE = 20

function getAngle(index: number, count: number): number {
  if (count <= 1) return 0
  const half = SPREAD_ANGLE / 2
  return -half + (SPREAD_ANGLE / (count - 1)) * index
}

function getLift(index: number, count: number): number {
  if (count <= 1) return 0
  const mid = (count - 1) / 2
  return Math.abs(index - mid) * 6
}

/** @package */
export function HandDisplay({ cards, selectedCard, disabled, onSelect, blockedCards = [] }: Props) {
  const { t } = useTranslation()
  const hintsEnabled = useHintsEnabled()
  if (cards.length === 0) {
    return (
      <div className={styles.empty}>
        <span>{t('rootaccess.noCards')}</span>
      </div>
    )
  }

  return (
    <div className={styles.hand} aria-label={t('rootaccess.yourHand')}>
      <AnimatePresence mode='popLayout'>
        {cards.map((card, i) => {
          const isBlocked = blockedCards.includes(card)
          const isSelected = selectedCard === card
          const angle = getAngle(i, cards.length)
          const lift = getLift(i, cards.length)

          return (
            <motion.div
              key={`${card}-${i}`}
              className={styles.cardSlot}
              layout={true}
              variants={dealVariants}
              initial='initial'
              animate='animate'
              exit='exit'
              style={{
                rotate: `${angle}deg`,
                translateY: isSelected ? lift - 12 : lift,
                zIndex: isSelected ? 10 : i,
              }}
              transition={springTransition}
            >
              <div className={isSelected ? styles.selectedGlow : undefined}>
                {hintsEnabled ? (
                  <TooltipWrapper text={`${CARD_META[card].label}: ${CARD_META[card].effect}`}>
                    <Card
                      front={
                        <div className={styles.face}>
                          <CardFace card={card} />
                        </div>
                      }
                      disabled={disabled || isBlocked}
                      onClick={!disabled && !isBlocked ? () => onSelect(card) : undefined}
                    />
                  </TooltipWrapper>
                ) : (
                  <Card
                    front={
                      <div className={styles.face}>
                        <CardFace card={card} />
                      </div>
                    }
                    disabled={disabled || isBlocked}
                    onClick={!disabled && !isBlocked ? () => onSelect(card) : undefined}
                  />
                )}
              </div>
              {isBlocked && hintsEnabled && <HintText text={t('rootaccess.mustPlay')} />}
              {isBlocked && !hintsEnabled && (
                <span className={styles.blockedLabel}>{t('rootaccess.mustPlay')}</span>
              )}
            </motion.div>
          )
        })}
      </AnimatePresence>
    </div>
  )
}
