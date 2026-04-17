import type { MotionProps } from 'motion/react'
import type { ReactNode, RefObject } from 'react'

// ---------------------------------------------------------------------------
// Card
// ---------------------------------------------------------------------------

export interface CardProps {
  front: ReactNode
  back?: ReactNode
  faceDown?: boolean
  disabled?: boolean
  onFlip?: (faceDown: boolean) => void
  onClick?: () => void
  motionProps?: MotionProps
}

// ---------------------------------------------------------------------------
// CardHand
// ---------------------------------------------------------------------------

export interface CardHandProps<T> {
  cards: T[]
  renderCard: (card: T) => ReactNode
  faceDown?: boolean
  spreadAngle?: number
  onPlay?: (card: T, index: number) => void
  onReveal?: (card: T, index: number) => void
  targetRef?: RefObject<HTMLElement | null>
}

// ---------------------------------------------------------------------------
// CardPile
// ---------------------------------------------------------------------------

export interface CardPileProps<T> {
  count: number
  faceDown?: boolean
  topCard?: T
  renderCard?: (card: T) => ReactNode
  onDraw?: () => void
  pileRef?: RefObject<HTMLElement | null>
}

// ---------------------------------------------------------------------------
// CardZone
// ---------------------------------------------------------------------------

export interface CardZoneEntry<T> {
  key: string
  data: T
  faceDown?: boolean
}

export interface CardZoneProps<T> {
  cards: CardZoneEntry<T>[]
  renderCard: (card: T, index: number) => ReactNode
  layout?: 'pile' | 'spread' | 'stack'
  zoneRef?: RefObject<HTMLElement | null>
  motionProps?: MotionProps
}
