import { useCallback, useRef } from 'react'
import { motion, AnimatePresence } from 'motion/react'
import { testId } from '@/utils/testId'
import { Card } from './Card'
import { dealVariants, springTransition, getFlyOutTarget } from './animations'
import type { CardHandProps } from './types'
import styles from './CardHand.module.css'

const DEFAULT_SPREAD = 30

export function CardHand<T>({
  cards,
  renderCard,
  faceDown = false,
  spreadAngle = DEFAULT_SPREAD,
  onPlay,
  onReveal,
  targetRef,
}: CardHandProps<T>) {
  const slotRefs = useRef<Map<number, HTMLElement>>(new Map())

  const getAngle = useCallback(
    (index: number) => {
      const count = cards.length
      if (count <= 1) return 0
      const half = spreadAngle / 2
      return -half + (spreadAngle / (count - 1)) * index
    },
    [cards.length, spreadAngle],
  )

  const getLift = useCallback(
    (index: number) => {
      const count = cards.length
      if (count <= 1) return 0
      const mid = (count - 1) / 2
      const distance = Math.abs(index - mid)
      return distance * 8
    },
    [cards.length],
  )

  const handlePlay = useCallback(
    async (card: T, index: number) => {
      const el = slotRefs.current.get(index)
      if (!el) {
        onPlay?.(card, index)
        return
      }

      const target = getFlyOutTarget(el, targetRef?.current)
      const motionEl = el.querySelector('[data-testid="card"]') as HTMLElement | null

      if (motionEl) {
        const { animate: motionAnimate } = await import('motion')
        await motionAnimate(
          motionEl,
          { x: target.x, y: target.y, opacity: 0, scale: 0.7 },
          { duration: 0.7, ease: [0.4, 0, 0.2, 1] },
        )
      }

      onPlay?.(card, index)
    },
    [onPlay, targetRef],
  )

  return (
    <div className={styles.hand} {...testId('card-hand')} aria-label='Your hand'>
      <AnimatePresence mode='popLayout'>
        {cards.map((card, index) => {
          const angle = getAngle(index)
          const lift = getLift(index)

          return (
            <motion.div
              key={index}
              ref={el => {
                if (el) slotRefs.current.set(index, el)
                else slotRefs.current.delete(index)
              }}
              className={styles.slot}
              layout={true}
              variants={dealVariants}
              initial='initial'
              animate='animate'
              exit='exit'
              style={{
                rotate: `${angle}deg`,
                translateY: lift,
                zIndex: index,
              }}
              transition={springTransition}
              {...testId(`card-hand-slot-${index}`)}
            >
              <Card
                front={renderCard(card)}
                faceDown={faceDown}
                onClick={() => {
                  if (faceDown) {
                    onReveal?.(card, index)
                  } else {
                    handlePlay(card, index)
                  }
                }}
              />
            </motion.div>
          )
        })}
      </AnimatePresence>
    </div>
  )
}
