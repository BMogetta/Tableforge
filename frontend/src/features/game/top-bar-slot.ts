import { createContext, useContext } from 'react'

/**
 * Lets a game renderer portal game-specific chips (round badge, set-aside,
 * anything else worth pinning to the session chrome) into the GameTopBar.
 *
 * Game.tsx creates the DOM element via GameTopBar's slotRef, stores it in
 * state, and publishes it through this context. The renderer then uses
 * createPortal(…, slot) to inject content. When mounted standalone (unit
 * tests) the slot is null and the renderer renders nothing — no-op fallback.
 */
export const GameTopBarSlotContext = createContext<HTMLElement | null>(null)

export function useGameTopBarSlot(): HTMLElement | null {
  return useContext(GameTopBarSlotContext)
}
