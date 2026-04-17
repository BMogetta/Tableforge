import type { MotionProps } from 'motion/react'
import type { ReactNode, RefObject } from 'react'

// ---------------------------------------------------------------------------
// Token
// ---------------------------------------------------------------------------

export interface TokenProps {
  children: ReactNode
  /** Which side has claimed this token. null = unclaimed (centered). */
  claimedBy?: 'top' | 'bottom' | null
  disabled?: boolean
  onClick?: () => void
  motionProps?: MotionProps
}

// ---------------------------------------------------------------------------
// TokenRow
// ---------------------------------------------------------------------------

export interface TokenRowProps<T> {
  tokens: T[]
  renderToken: (token: T, index: number) => ReactNode
  /** Return 'top', 'bottom', or null for each token's claim state. */
  getClaimState: (token: T, index: number) => 'top' | 'bottom' | null
  /** Distance in px the token slides when claimed. Default 40. */
  claimOffset?: number
  rowRef?: RefObject<HTMLElement | null>
  motionProps?: MotionProps
}
