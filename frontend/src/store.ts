import { create } from 'zustand'
import type { Player } from './api'
import { RoomSocket } from './ws'

interface AppState {
  player: Player | null
  setPlayer: (p: Player | null) => void

  socket: RoomSocket | null
  joinRoom: (roomId: string) => void
  leaveRoom: () => void

  pendingRematch: string | null
  setPendingRematch: (sessionId: string | null) => void
}

export const useAppStore = create<AppState>((set, get) => ({
  player: null,
  setPlayer: (player) => set({ player }),

  socket: null,
  joinRoom: (roomId: string) => {
    // Close existing socket if switching rooms.
    get().socket?.close()
    const socket = new RoomSocket(roomId)
    socket.connect()
    set({ socket })
  },
  leaveRoom: () => {
    get().socket?.close()
    set({ socket: null })
  },
  pendingRematch: null,
  setPendingRematch: (sessionId) => set({ pendingRematch: sessionId }),
}))