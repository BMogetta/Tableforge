import { create } from 'zustand'
import type { Player } from './api'

interface AppState {
  player: Player | null
  setPlayer: (p: Player | null) => void
}

export const useAppStore = create<AppState>((set) => ({
  player: null,
  setPlayer: (player) => set({ player }),
}))
