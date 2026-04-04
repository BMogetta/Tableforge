import { useCallback, useRef, useImperativeHandle } from 'react'
import { motion, AnimatePresence } from 'motion/react'
import { testId } from '@/utils/testId'
import { Card } from './Card'
import { dealVariants, springTransition } from './animations'
import type { CardPileProps } from './types'
import styles from './CardPile.module.css'

const STACK_OFFSETS = [
  { x: 0, y: 0 },
  { x: 2, y: -2 },
  { x: 4, y: -4 },
]

export function CardPile<T>({
  count,
  faceDown = true,
  topCard,
  renderCard,
  onDraw,
  pileRef,
}: CardPileProps<T>) {
  const containerRef = useRef<HTMLDivElement>(null)

  // Expose the container element via pileRef so other components can target it
  useImperativeHandle(pileRef, () => containerRef.current!, [])

  const isEmpty = count === 0
  const visibleLayers = Math.min(count, STACK_OFFSETS.length)

  const handleClick = useCallback(() => {
    if (isEmpty) return
    onDraw?.()
  }, [isEmpty, onDraw])

  return (
    <div ref={containerRef} className={styles.pile} data-empty={isEmpty} {...testId('card-pile')}>
      <AnimatePresence mode='popLayout'>
        {Array.from({ length: visibleLayers }, (_, i) => {
          const isTop = i === visibleLayers - 1
          const offset = STACK_OFFSETS[i]

          return (
            <motion.div
              key={`layer-${i}`}
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
              {isTop && topCard && renderCard ? (
                <Card front={renderCard(topCard)} faceDown={faceDown} onClick={handleClick} />
              ) : (
                <Card
                  front={<div />}
                  faceDown={true}
                  onClick={isTop ? handleClick : undefined}
                  disabled={!isTop}
                />
              )}
            </motion.div>
          )
        })}
      </AnimatePresence>
      {count > 0 && (
        <span className={styles.count} aria-label={`${count} cards remaining`}>
          {count}
        </span>
      )}
    </div>
  )
}
