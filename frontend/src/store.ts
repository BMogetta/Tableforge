import { create } from 'zustand'
import type { Player } from './api'
import { RoomSocket } from './ws'

interface AppState {
  player: Player | null
  setPlayer: (p: Player | null) => void

  socket: RoomSocket | null
  /**
   * Opens a WebSocket connection for the given room.
   * @param roomId  The room UUID — used to identify the active room.
   * @param wsUrl   Full WebSocket URL (including player_id query param).
   *                Build this with wsRoomUrl() from api.ts.
   */
  joinRoom: (roomId: string, wsUrl: string) => void
  leaveRoom: () => void
}

export const useAppStore = create<AppState>((set, get) => ({
  player: null,
  setPlayer: (player) => set({ player }),

  socket: null,
  joinRoom: (roomId: string, wsUrl: string) => {
    // Close existing socket if switching rooms.
    get().socket?.close()
    const socket = new RoomSocket(wsUrl)
    socket.connect()
    set({ socket })
  },
  leaveRoom: () => {
    get().socket?.close()
    set({ socket: null })
  },
}))