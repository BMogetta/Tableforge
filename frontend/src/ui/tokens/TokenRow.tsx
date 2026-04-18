import { motion } from 'motion/react'
import { useImperativeHandle, useRef } from 'react'
import { testId } from '@/utils/testId'
import { getClaimVariants, tokenSpring } from './animations'
import { Token } from './Token'
import styles from './TokenRow.module.css'
import type { TokenRowProps } from './types'

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
    // biome-ignore lint/a11y/useSemanticElements: fieldset would break row layout styling
    <div
      ref={containerRef}
      className={styles.row}
      role='group'
      aria-label='Token row'
      {...testId('token-row')}
    >
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
