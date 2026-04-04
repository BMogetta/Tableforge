import { type RefObject } from 'react'
import { motion, AnimatePresence } from 'motion/react'
import { Card } from '@/ui/cards'
import { dealVariants, springTransition } from '@/ui/cards'
import { CardFace } from './CardFace'
import type { CardName } from './CardDisplay'
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

export function HandDisplay({
  cards,
  selectedCard,
  disabled,
  onSelect,
  blockedCards = [],
}: Props) {
  if (cards.length === 0) {
    return (
      <div className={styles.empty}>
        <span>No cards in hand</span>
      </div>
    )
  }

  return (
    <div className={styles.hand} aria-label="Your hand">
      <AnimatePresence mode="popLayout">
        {cards.map((card, i) => {
          const isBlocked = blockedCards.includes(card)
          const isSelected = selectedCard === card
          const angle = getAngle(i, cards.length)
          const lift = getLift(i, cards.length)

          return (
            <motion.div
              key={`${card}-${i}`}
              className={styles.cardSlot}
              layout
              variants={dealVariants}
              initial="initial"
              animate="animate"
              exit="exit"
              style={{
                rotate: `${angle}deg`,
                translateY: isSelected ? lift - 12 : lift,
                zIndex: isSelected ? 10 : i,
              }}
              transition={springTransition}
            >
              <div className={isSelected ? styles.selectedGlow : undefined}>
                <Card
                  front={
                    <div className={styles.face}>
                      <CardFace card={card} />
                    </div>
                  }
                  disabled={disabled || isBlocked}
                  onClick={!disabled && !isBlocked ? () => onSelect(card) : undefined}
                />
              </div>
              {isBlocked && <span className={styles.blockedLabel}>Must play ENCRYPTED_KEY</span>}
            </motion.div>
          )
        })}
      </AnimatePresence>
    </div>
  )
}
