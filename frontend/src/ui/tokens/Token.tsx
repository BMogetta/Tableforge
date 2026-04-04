import { motion } from 'motion/react'
import { testId } from '@/utils/testId'
import { getClaimVariants, tokenSpring } from './animations'
import type { TokenProps } from './types'
import styles from './Token.module.css'

const DEFAULT_CLAIM_OFFSET = 40

export function Token({
  children,
  claimedBy = null,
  disabled = false,
  onClick,
  motionProps,
}: TokenProps) {
  const isClaimed = claimedBy !== null
  const isClickable = !disabled && !!onClick

  const variants = getClaimVariants(DEFAULT_CLAIM_OFFSET)
  const animateState = claimedBy ?? 'unclaimed'

  return (
    <motion.button
      type='button'
      className={[
        styles.token,
        isClaimed ? styles.claimed : '',
        isClickable ? styles.clickable : '',
      ]
        .filter(Boolean)
        .join(' ')}
      data-disabled={disabled}
      variants={variants}
      animate={animateState}
      transition={tokenSpring}
      onClick={isClickable ? onClick : undefined}
      aria-label={isClaimed ? `Token claimed by ${claimedBy}` : 'Unclaimed token'}
      {...testId('token')}
      {...motionProps}
    >
      {children}
    </motion.button>
  )
}
