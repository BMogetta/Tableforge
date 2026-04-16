import { create } from 'zustand'
import { useSocketStore } from './socketStore'

interface RoomState {
  activeRoomId: string | null
  roomSocketStatus: 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

  isSpectator: boolean
  setIsSpectator: (value: boolean) => void

  spectatorCount: number
  setSpectatorCount: (count: number) => void

  presenceMap: Record<string, boolean>
  setPlayerPresence: (playerId: string, online: boolean) => void

  joinRoom: (roomId: string) => void
  leaveRoom: () => void
  handleMatchReady: (roomId: string) => void
}

export const useRoomStore = create<RoomState>((set, get) => ({
  activeRoomId: null,
  roomSocketStatus: 'disconnected',

  isSpectator: false,
  setIsSpectator: value => set({ isSpectator: value }),

  spectatorCount: 0,
  setSpectatorCount: count => set({ spectatorCount: count }),

  presenceMap: {},
  setPlayerPresence: (playerId, online) =>
    set(state => ({
      presenceMap: { ...state.presenceMap, [playerId]: online },
    })),

  joinRoom: (roomId: string) => {
    if (get().activeRoomId === roomId) return
    const gw = useSocketStore.getState().gateway
    if (get().activeRoomId) {
      gw?.unsubscribeRoom()
    }
    gw?.subscribeRoom(roomId)
    set({
      activeRoomId: roomId,
      roomSocketStatus: 'connecting',
      isSpectator: false,
      spectatorCount: 0,
      presenceMap: {},
    })
  },

  leaveRoom: () => {
    useSocketStore.getState().gateway?.unsubscribeRoom()
    set({
      activeRoomId: null,
      roomSocketStatus: 'disconnected',
      isSpectator: false,
      spectatorCount: 0,
      presenceMap: {},
    })
  },

  handleMatchReady: (roomId: string) => {
    // Queue clearing is done by the caller (useQueueStore.clearQueue)
    get().joinRoom(roomId)
  },
}))

// --- Central WS event routing for room-level events -------------------------
// Registered once when the socketStore's gateway changes.
let unsubscribe: (() => void) | null = null

useSocketStore.subscribe((state, prev) => {
  if (state.gateway === prev.gateway) return

  // Clean up previous listener.
  unsubscribe?.()
  unsubscribe = null

  const gw = state.gateway
  if (!gw) return

  unsubscribe = gw.on(event => {
    const store = useRoomStore.getState()
    if (event.type === 'ws_connected') useRoomStore.setState({ roomSocketStatus: 'connected' })
    if (event.type === 'ws_reconnecting') useRoomStore.setState({ roomSocketStatus: 'reconnecting' })
    if (event.type === 'ws_disconnected') useRoomStore.setState({ roomSocketStatus: 'disconnected' })
    if (event.type === 'room_subscribed') useRoomStore.setState({ roomSocketStatus: 'connected' })
    if (event.type === 'presence_update') {
      store.setPlayerPresence(event.payload.player_id, event.payload.online)
    }
  })
})
