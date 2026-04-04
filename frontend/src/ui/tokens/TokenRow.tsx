import { useRef, useImperativeHandle } from 'react'
import { motion } from 'motion/react'
import { testId } from '@/utils/testId'
import { Token } from './Token'
import { getClaimVariants, tokenSpring } from './animations'
import type { TokenRowProps } from './types'
import styles from './TokenRow.module.css'

const DEFAULT_CLAIM_OFFSET = 40

export function TokenRow<T>({
  tokens,
  renderToken,
  getClaimState,
  claimOffset = DEFAULT_CLAIM_OFFSET,
  rowRef,
  motionProps,
}: TokenRowProps<T>) {
  const containerRef = useRef<HTMLDivElement>(null)

  useImperativeHandle(rowRef, () => containerRef.current!, [])

  const variants = getClaimVariants(claimOffset)

  return (
    <div ref={containerRef} className={styles.row} aria-label='Token row' {...testId('token-row')}>
      {tokens.map((token, index) => {
        const claimState = getClaimState(token, index)
        const animateState = claimState ?? 'unclaimed'

        return (
          <motion.div
            key={index}
            variants={variants}
            animate={animateState}
            transition={tokenSpring}
            {...motionProps}
          >
            <Token claimedBy={claimState}>{renderToken(token, index)}</Token>
          </motion.div>
        )
      })}
    </div>
  )
}
