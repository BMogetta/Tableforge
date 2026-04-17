import { AnimatePresence, motion } from 'motion/react'
import { useImperativeHandle, useRef } from 'react'
import { testId } from '@/utils/testId'
import { flyInVariants, slideInVariants, springTransition } from './animations'
import { Card } from './Card'
import styles from './CardZone.module.css'
import type { CardZoneProps } from './types'

export function CardZone<T>({
  cards,
  renderCard,
  layout: layoutMode = 'pile',
  zoneRef,
  motionProps,
}: CardZoneProps<T>) {
  const containerRef = useRef<HTMLDivElement>(null)

  useImperativeHandle(zoneRef, () => containerRef.current!, [])

  return (
    <div
      ref={containerRef}
      className={styles[layoutMode]}
      aria-label='Card zone'
      {...testId('card-zone')}
    >
      <AnimatePresence>
        {cards.map((entry, index) => (
          <motion.div
            key={entry.key}
            className={styles.entry}
            variants={layoutMode === 'stack' ? slideInVariants : flyInVariants}
            initial='initial'
            animate='animate'
            exit='exit'
            transition={springTransition}
            style={layoutMode === 'pile' ? { zIndex: index } : undefined}
            {...motionProps}
          >
            <Card front={renderCard(entry.data, index)} faceDown={entry.faceDown} disabled={true} />
          </motion.div>
        ))}
      </AnimatePresence>
    </div>
  )
}
