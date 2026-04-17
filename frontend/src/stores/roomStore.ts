import { create } from 'zustand'
import type { RoomView } from '@/lib/api'
import { useSocketStore } from './socketStore'

interface RoomState {
  activeRoomId: string | null
  roomSocketStatus: 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

  view: RoomView | null
  setView: (view: RoomView) => void

  ownerId: string | null
  setOwnerId: (id: string | null) => void

  settings: Record<string, string>
  updateSetting: (key: string, value: string) => void

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

const roomDefaults = {
  view: null as RoomView | null,
  ownerId: null as string | null,
  settings: {} as Record<string, string>,
  isSpectator: false,
  spectatorCount: 0,
  presenceMap: {} as Record<string, boolean>,
}

export const useRoomStore = create<RoomState>((set, get) => ({
  activeRoomId: null,
  roomSocketStatus: 'disconnected',

  ...roomDefaults,

  setView: view => set({ view }),
  setOwnerId: id => set({ ownerId: id }),
  updateSetting: (key, value) => set(state => ({ settings: { ...state.settings, [key]: value } })),

  setIsSpectator: value => set({ isSpectator: value }),
  setSpectatorCount: count => set({ spectatorCount: count }),
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
      ...roomDefaults,
    })
  },

  leaveRoom: () => {
    useSocketStore.getState().gateway?.unsubscribeRoom()
    set({
      activeRoomId: null,
      roomSocketStatus: 'disconnected',
      ...roomDefaults,
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
    if (event.type === 'ws_reconnecting')
      useRoomStore.setState({ roomSocketStatus: 'reconnecting' })
    if (event.type === 'ws_disconnected')
      useRoomStore.setState({ roomSocketStatus: 'disconnected' })
    if (event.type === 'room_subscribed') useRoomStore.setState({ roomSocketStatus: 'connected' })
    if (event.type === 'presence_update') {
      store.setPlayerPresence(event.payload.player_id, event.payload.online)
    }
    if (event.type === 'owner_changed') {
      store.setOwnerId(event.payload.owner_id)
    }
    if (event.type === 'setting_updated') {
      store.updateSetting(event.payload.key, event.payload.value)
    }
    if (event.type === 'spectator_joined' || event.type === 'spectator_left') {
      store.setSpectatorCount(event.payload.spectator_count)
    }
  })
})
