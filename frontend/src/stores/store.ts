import { create } from 'zustand'
import type { PlayerSettingMap, PlayerSettings } from '@/lib/api'
import { DEFAULT_SETTINGS } from '@/lib/api'
import type { Player } from '@/lib/schema-generated.zod'
import { GatewaySocket } from '@/lib/ws'
import { applyFontSize, applySkin, type FontSize, type SkinId } from '@/lib/skins'
import { i18n } from '@/lib/i18n'

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

export const QueueStatus = {
  Idle: 'idle',
  Queued: 'queued',
  MatchFound: 'match_found',
} as const
export type QueueStatus = (typeof QueueStatus)[keyof typeof QueueStatus]

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

// Resolved settings — all keys are guaranteed non-optional after merging
// stored values over DEFAULT_SETTINGS.
export type ResolvedSettings = Required<PlayerSettingMap>

interface AppState {
  player: Player | null
  setPlayer: (p: Player | null) => void

  /**
   * Unified WebSocket connection. Connects once at login, closed on logout.
   * Handles both player-scoped events (DMs, queue, notifications) and
   * room-scoped events (game moves, chat, presence) on a single connection.
   * Room subscriptions are managed via control messages (subscribeRoom /
   * unsubscribeRoom).
   */
  gateway: GatewaySocket | null
  connectGateway: (url: string) => void
  disconnectGateway: () => void

  activeRoomId: string | null

  /**
   * Connection status of the gateway socket. Updated centrally so components
   * read from the store instead of subscribing independently.
   */
  roomSocketStatus: 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

  /**
   * True when the current player joined as a spectator (not in room_players).
   * Set by Room.tsx once the room view loads and the player list is known.
   * Consumed by Game.tsx to hide participant-only UI (rematch button, etc).
   */
  isSpectator: boolean
  setIsSpectator: (value: boolean) => void

  /**
   * Live count of spectators currently watching the room.
   * Updated via spectator_joined / spectator_left WS events in Room.tsx.
   * Read by Room.tsx and Game.tsx to display the "N watching" badge.
   */
  spectatorCount: number
  setSpectatorCount: (count: number) => void

  /**
   * Subscribes the gateway to a room channel. The server sends a
   * room_subscribed ack and starts delivering room events.
   */
  joinRoom: (roomId: string) => void
  leaveRoom: () => void

  /**
   * Called when the server confirms a ranked match is ready.
   * Clears queue state and subscribes to the room — the component only needs
   * to navigate afterward.
   */
  handleMatchReady: (roomId: string) => void

  /**
   * Live presence map: playerID → online.
   * Updated via presence_update WS events handled centrally by the gateway
   * event listener registered in connectGateway.
   */
  presenceMap: Record<string, boolean>
  setPlayerPresence: (playerId: string, online: boolean) => void

  // --- Queue state -----------------------------------------------------------

  /**
   * Current matchmaking queue status for the player.
   * Persists across navigation so the player can queue and browse the app.
   */
  queueStatus: QueueStatus

  /**
   * Timestamp (Date.now()) when the player joined the queue.
   * Used to calculate elapsed wait time from any page.
   * Null when queueStatus is QueueStatus.Idle.
   */
  queueJoinedAt: number | null

  /**
   * The match ID proposed by the server when queueStatus is QueueStatus.MatchFound.
   * Required to call queue.accept() / queue.decline().
   * Null otherwise.
   */
  matchId: string | null

  /** Set queue status to QueueStatus.Queued and record the join timestamp. */
  setQueued: (joinedAt: number) => void
  /** Set queue status to QueueStatus.MatchFound and record the match ID. */
  setMatchFound: (matchId: string) => void
  clearQueue: () => void

  // --- DM target (global) ----------------------------------------------------

  /** When set, the DM overlay opens a conversation with this player. */
  dmTarget: string | null
  setDmTarget: (id: string | null) => void

  // --- Settings state --------------------------------------------------------

  /**
   * Resolved player settings — stored values merged over DEFAULT_SETTINGS.
   * All keys are guaranteed non-optional.
   *
   * Load order on app start:
   *   1. localStorage cache (instant, no flash)
   *   2. Backend fetch in background (updates store + refreshes cache)
   *
   * On change:
   *   1. Optimistic update in store (immediate)
   *   2. localStorage write (immediate)
   *   3. Debounced PUT to backend (500ms)
   */
  settings: ResolvedSettings

  /**
   * Hydrates the settings store from a raw PlayerSettings API response.
   * Merges stored values over DEFAULT_SETTINGS so all keys are always present.
   */
  hydrateSettings: (raw: PlayerSettings) => void

  /**
   * Applies a partial settings update optimistically.
   * Callers are responsible for debouncing the backend sync.
   */
  updateSetting: <K extends keyof PlayerSettingMap>(key: K, value: PlayerSettingMap[K]) => void

  /**
   * Replaces all settings at once — used when loading from localStorage cache.
   */
  setSettings: (settings: ResolvedSettings) => void
}

export const useAppStore = create<AppState>((set, get) => ({
  player: null,
  setPlayer: player => set({ player }),

  gateway: null,
  connectGateway: (url: string) => {
    const existing = get().gateway
    if (existing && existing.url === url) return
    existing?.close()
    const gw = new GatewaySocket(url)

    // Central handler for connection status + presence updates.
    gw.on(event => {
      if (event.type === 'ws_connected') set({ roomSocketStatus: 'connected' })
      if (event.type === 'ws_reconnecting') set({ roomSocketStatus: 'reconnecting' })
      if (event.type === 'ws_disconnected') set({ roomSocketStatus: 'disconnected' })
      if (event.type === 'room_subscribed') set({ roomSocketStatus: 'connected' })
      if (event.type === 'presence_update') {
        get().setPlayerPresence(event.payload.player_id, event.payload.online)
      }
    })

    gw.connect()
    set({ gateway: gw })
  },
  disconnectGateway: () => {
    get().gateway?.close()
    set({ gateway: null })
  },

  activeRoomId: null,
  roomSocketStatus: 'connecting',

  isSpectator: false,
  setIsSpectator: value => set({ isSpectator: value }),

  spectatorCount: 0,
  setSpectatorCount: count => set({ spectatorCount: count }),

  joinRoom: (roomId: string) => {
    if (get().activeRoomId === roomId) return
    // Unsubscribe from previous room if any.
    const gw = get().gateway
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
    get().gateway?.unsubscribeRoom()
    set({
      activeRoomId: null,
      roomSocketStatus: 'disconnected',
      isSpectator: false,
      spectatorCount: 0,
      presenceMap: {},
    })
  },
  handleMatchReady: (roomId: string) => {
    get().clearQueue()
    get().joinRoom(roomId)
  },

  presenceMap: {},
  setPlayerPresence: (playerId, online) =>
    set(state => ({
      presenceMap: { ...state.presenceMap, [playerId]: online },
    })),

  // Queue
  queueStatus: QueueStatus.Idle,
  queueJoinedAt: null,
  matchId: null,
  setQueued: joinedAt =>
    set({ queueStatus: QueueStatus.Queued, queueJoinedAt: joinedAt, matchId: null }),
  setMatchFound: matchId => set({ queueStatus: QueueStatus.MatchFound, matchId }),
  clearQueue: () => set({ queueStatus: QueueStatus.Idle, queueJoinedAt: null, matchId: null }),

  dmTarget: null,
  setDmTarget: id => set({ dmTarget: id }),

  // Settings
  settings: { ...DEFAULT_SETTINGS },

  hydrateSettings: (raw: PlayerSettings) => {
    const merged: ResolvedSettings = { ...DEFAULT_SETTINGS, ...raw.settings }
    set({ settings: merged })
    if (merged.theme) applySkin(merged.theme as SkinId)
    if (merged.font_size) applyFontSize(merged.font_size as FontSize)
    if (merged.language) i18n.changeLanguage(merged.language)
  },

  updateSetting: (key, value) => {
    set(state => ({
      settings: { ...state.settings, [key]: value },
    }))
    if (key === 'theme') applySkin(value as SkinId)
    if (key === 'font_size') applyFontSize(value as FontSize)
    if (key === 'language') i18n.changeLanguage(value as string)
  },

  setSettings: settings => set({ settings }),
}))
