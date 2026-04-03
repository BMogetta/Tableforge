import type { Transition, Variants } from 'motion/react'

// ---------------------------------------------------------------------------
// Shared transition
// ---------------------------------------------------------------------------

export const springTransition: Transition = {
  type: 'spring',
  stiffness: 300,
  damping: 25,
}

// ---------------------------------------------------------------------------
// Flip
// ---------------------------------------------------------------------------

export const flipVariants: Variants = {
  faceUp: { rotateY: 0 },
  faceDown: { rotateY: 180 },
}

export const flipTransition: Transition = {
  duration: 0.5,
  ease: [0.4, 0, 0.2, 1],
}

// ---------------------------------------------------------------------------
// Lift (hover)
// ---------------------------------------------------------------------------

export const liftWhileHover = {
  y: -12,
  scale: 1.05,
  transition: springTransition,
}

// ---------------------------------------------------------------------------
// flyOut — compute motion values toward a target element
// ---------------------------------------------------------------------------

export function getFlyOutTarget(
  sourceEl: HTMLElement,
  targetEl: HTMLElement | null | undefined,
): { x: number; y: number } {
  if (!targetEl) {
    return { x: 0, y: -window.innerHeight }
  }

  const sourceRect = sourceEl.getBoundingClientRect()
  const targetRect = targetEl.getBoundingClientRect()

  return {
    x: targetRect.left + targetRect.width / 2 - (sourceRect.left + sourceRect.width / 2),
    y: targetRect.top + targetRect.height / 2 - (sourceRect.top + sourceRect.height / 2),
  }
}

// ---------------------------------------------------------------------------
// flyIn
// ---------------------------------------------------------------------------

export const flyInVariants: Variants = {
  initial: { opacity: 0, scale: 0.6, y: -40 },
  animate: { opacity: 1, scale: 1, y: 0 },
  exit: { opacity: 0, scale: 0.6, y: 40 },
}

// ---------------------------------------------------------------------------
// Deal (pile → hand)
// ---------------------------------------------------------------------------

export const dealVariants: Variants = {
  initial: { opacity: 0, scale: 0.8 },
  animate: { opacity: 1, scale: 1 },
  exit: { opacity: 0, scale: 0.8, y: -30 },
}
