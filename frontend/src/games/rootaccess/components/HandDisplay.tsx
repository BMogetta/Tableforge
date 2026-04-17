import { AnimatePresence, motion } from 'motion/react'
import type { RefObject } from 'react'
import { useCallback, useEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { Card, dealVariants, getFlyOutTarget, springTransition } from '@/ui/cards'
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
  /**
   * When set, the matching card is animated flying to targetRef before the
   * parent triggers the move mutation. onPlayComplete fires after the fly-out
   * completes (or immediately if the target is unreachable).
   */
  playingCard?: CardName | null
  onPlayComplete?: () => void
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
export function HandDisplay({
  cards,
  selectedCard,
  disabled,
  onSelect,
  blockedCards = [],
  targetRef,
  playingCard,
  onPlayComplete,
}: Props) {
  const { t } = useTranslation()
  const hintsEnabled = useHintsEnabled()
  const slotRefs = useRef<Map<number, HTMLElement>>(new Map())

  const setSlotRef = useCallback(
    (index: number) => (el: HTMLElement | null) => {
      if (el) slotRefs.current.set(index, el)
      else slotRefs.current.delete(index)
    },
    [],
  )

  // Fly-out: when playingCard is set, animate the matching slot toward targetRef,
  // then notify the parent so it can fire the mutation.
  useEffect(() => {
    if (!playingCard || !onPlayComplete) return
    const idx = cards.findIndex(c => c === playingCard)
    if (idx < 0) {
      onPlayComplete()
      return
    }
    const slot = slotRefs.current.get(idx)
    const motionEl = slot?.querySelector('[data-testid="card"]') as HTMLElement | null
    if (!slot || !motionEl) {
      onPlayComplete()
      return
    }

    let cancelled = false
    // Safety timeout — if motion never resolves (hung import, animation stuck),
    // force the parent to unblock so the user can still play or cancel.
    const safety = window.setTimeout(() => {
      if (cancelled) return
      motionEl.style.transform = ''
      motionEl.style.opacity = ''
      onPlayComplete()
      cancelled = true
    }, 1500)
    ;(async () => {
      try {
        const target = getFlyOutTarget(slot, targetRef?.current)
        const { animate: motionAnimate } = await import('motion')
        await motionAnimate(
          motionEl,
          { x: target.x, y: target.y, opacity: 0, scale: 0.7 },
          { duration: 0.7, ease: [0.4, 0, 0.2, 1] },
        )
      } catch {
        // Swallow — the finally block + safety timeout handle cleanup.
      } finally {
        window.clearTimeout(safety)
        // Clear inline styles so if React reuses this DOM node for a
        // different card (e.g. another copy of the same card remaining
        // in hand under the same key), the next render isn't stuck at
        // opacity:0 / translated offscreen.
        motionEl.style.transform = ''
        motionEl.style.opacity = ''
        if (!cancelled) onPlayComplete()
      }
    })()

    return () => {
      cancelled = true
      window.clearTimeout(safety)
      motionEl.style.transform = ''
      motionEl.style.opacity = ''
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [playingCard])

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
              ref={setSlotRef(i)}
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
              transition={{ ...springTransition, delay: i * 0.35 }}
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
