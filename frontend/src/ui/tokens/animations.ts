import type { Transition, Variants } from 'motion/react'

// ---------------------------------------------------------------------------
// Shared transition
// ---------------------------------------------------------------------------

export const tokenSpring: Transition = {
  type: 'spring',
  stiffness: 250,
  damping: 20,
}

// ---------------------------------------------------------------------------
// Claim — token slides toward the claiming side
// ---------------------------------------------------------------------------

export function getClaimVariants(offset: number): Variants {
  return {
    unclaimed: { y: 0, scale: 1 },
    top: { y: -offset, scale: 1.05 },
    bottom: { y: offset, scale: 1.05 },
  }
}

// ---------------------------------------------------------------------------
// Pulse — brief glow on claim (used as a one-shot animation)
// ---------------------------------------------------------------------------

export const claimPulse = {
  scale: [1, 1.15, 1.05],
  transition: {
    duration: 0.4,
    ease: [0.4, 0, 0.2, 1],
  },
}
