/**
 * Turn notification system for hidden tabs.
 *
 * Sends a browser Notification + plays a sound when it becomes the player's
 * turn and the tab is not visible. Integrates with the existing SFX engine.
 *
 * Usage:
 *   1. Call `requestPermission()` once on game start.
 *   2. Call `notifyTurn()` when a move_applied event arrives and it's now
 *      the local player's turn.
 *
 * The WakeLock keeps the screen awake during the player's turn to prevent
 * the timer from ticking away while the screen is off (mobile).
 */

import { sfx } from './sfx'

// -- Permission ---------------------------------------------------------------

/** Ask for notification permission. No-op if already granted or denied. */
export async function requestPermission(): Promise<NotificationPermission> {
  if (!('Notification' in window)) return 'denied'
  if (Notification.permission !== 'default') return Notification.permission
  return Notification.requestPermission()
}

// -- Turn notification --------------------------------------------------------

/**
 * Show a system notification + play sound when the tab is hidden.
 * When the tab is visible, this is a no-op (the UI itself is enough).
 */
export function notifyTurn(): void {
  if (!document.hidden) return

  if ('Notification' in window && Notification.permission === 'granted') {
    new Notification("It's your turn!", {
      body: 'Make your move in Recess',
      icon: '/icon-192.png',
      tag: 'turn-notification',
    })
  }

  sfx.play('game.my_turn' as Parameters<typeof sfx.play>[0])
}

// -- WakeLock -----------------------------------------------------------------

let wakeLock: WakeLockSentinel | null = null

/** Request a screen wake lock. No-op if not supported or already held. */
export async function acquireWakeLock(): Promise<void> {
  if (wakeLock) return
  if (!('wakeLock' in navigator)) return
  try {
    wakeLock = await navigator.wakeLock.request('screen')
    wakeLock.addEventListener('release', () => {
      wakeLock = null
    })
  } catch {
    // WakeLock can fail if the tab is not visible — safe to ignore.
  }
}

/** Release the screen wake lock. */
export async function releaseWakeLock(): Promise<void> {
  if (!wakeLock) return
  await wakeLock.release()
  wakeLock = null
}
