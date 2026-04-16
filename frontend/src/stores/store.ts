import { create } from 'zustand'
import type { Player } from '@/lib/schema-generated.zod'

// Re-export from dedicated stores so existing `import { X } from '@/stores/store'`
// continues to work during migration. New code should import from the specific store.
export { QueueStatus, useQueueStore } from './queueStore'
export type { QueueStatus as QueueStatusType } from './queueStore'
export { useSocketStore } from './socketStore'
export { useRoomStore } from './roomStore'
export { useSettingsStore, type ResolvedSettings } from './settingsStore'

interface AppState {
  player: Player | null
  setPlayer: (p: Player | null) => void

  /** When set, the DM overlay opens a conversation with this player. */
  dmTarget: string | null
  setDmTarget: (id: string | null) => void
}

export const useAppStore = create<AppState>(set => ({
  player: null,
  setPlayer: player => set({ player }),

  dmTarget: null,
  setDmTarget: id => set({ dmTarget: id }),
}))
