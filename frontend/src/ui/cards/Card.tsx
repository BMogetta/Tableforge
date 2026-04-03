import { motion } from 'motion/react'
import { testId } from '@/utils/testId'
import { flipVariants, flipTransition, liftWhileHover } from './animations'
import type { CardProps } from './types'
import styles from './Card.module.css'

export function Card({
  front,
  back,
  faceDown = false,
  disabled = false,
  onFlip,
  onClick,
  motionProps,
}: CardProps) {
  function handleClick() {
    if (disabled) return
    onClick?.()
    onFlip?.(!faceDown)
  }

  return (
    <div className={styles.perspective}>
      <motion.button
        className={styles.card}
        data-disabled={disabled}
        type="button"
        variants={flipVariants}
        animate={faceDown ? 'faceDown' : 'faceUp'}
        transition={flipTransition}
        whileHover={disabled ? undefined : liftWhileHover}
        onClick={handleClick}
        aria-label={faceDown ? 'Face-down card' : 'Face-up card'}
        {...testId('card')}
        {...motionProps}
      >
        <div className={styles.front}>{front}</div>
        <div className={styles.back}>
          {back ?? <div className={styles.defaultBack} />}
        </div>
      </motion.button>
    </div>
  )
}
