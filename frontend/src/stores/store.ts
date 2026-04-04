import { create } from 'zustand'
import type { PlayerSettingMap, PlayerSettings } from '@/lib/api'
import type { Player } from '@/lib/schema-generated.zod'
import { DEFAULT_SETTINGS } from '@/lib/api'
import { PlayerSocket, RoomSocket } from '@/lib/ws'
import { applyFontSize, applySkin, type FontSize, type SkinId } from '@/lib/skins'

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

  socket: RoomSocket | null
  activeRoomId: string | null

  /**
   * Persistent WebSocket for the player's personal channel.
   * Receives queue events (match_found, match_cancelled, match_ready),
   * DM events, and notification_received — regardless of which room the
   * player is currently in.
   * Connected on login via connectPlayerSocket(), closed on logout.
   */
  playerSocket: PlayerSocket | null
  connectPlayerSocket: (url: string) => void
  disconnectPlayerSocket: () => void

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
   * Read by Room.tsx and Game.tsx to display the "👁 N watching" badge.
   */
  spectatorCount: number
  setSpectatorCount: (count: number) => void

  /**
   * Opens a WebSocket connection for the given room.
   * @param roomId  The room UUID — used to identify the active room.
   * @param wsUrl   Full WebSocket URL (including player_id query param).
   *                Build this with wsRoomUrl() from api.ts.
   */
  joinRoom: (roomId: string, wsUrl: string) => void
  leaveRoom: () => void

  /**
   * Live presence map: playerID → online.
   * Updated via presence_update WS events in Room.tsx and Game.tsx.
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

// NOTE: Settings persistence is currently handled manually via localStorage
// (see App.tsx: loadSettingsFromCache / saveSettingsToCache) and a debounced
// PUT to the backend in Settings.tsx.
//
// An alternative would be zustand/middleware `persist`:
//
//   import { persist } from 'zustand/middleware'
//   export const useAppStore = create(persist(
//     (set, get) => ({ ... }),
//     {
//       name: 'tf:settings',
//       partialize: (state) => ({ settings: state.settings }),
//     }
//   ))
//
// Implications of switching to `persist`:
//   + Eliminates the manual localStorage read/write boilerplate
//   + Built-in rehydration on store init (no need for setSettings in App.tsx)
//   + Supports versioning (migrate: fn) for schema changes
//   - Requires extracting settings into a separate store — `partialize` works
//     but mixing volatile state (sockets, presenceMap) with persisted state
//     in the same store is fragile and harder to reason about
//   - The debounced backend sync in Settings.tsx still needs to exist regardless
//     (persist only handles localStorage, not cross-device sync)
//
// Recommended path: if the store grows further, extract a dedicated
// useSettingsStore with `persist`, and keep useAppStore for volatile state.
export const useAppStore = create<AppState>((set, get) => ({
  player: null,
  setPlayer: player => set({ player }),

  socket: null,
  activeRoomId: null,

  playerSocket: null,
  connectPlayerSocket: (url: string) => {
    get().playerSocket?.close()
    const playerSocket = new PlayerSocket(url)
    playerSocket.connect()
    set({ playerSocket })
  },
  disconnectPlayerSocket: () => {
    get().playerSocket?.close()
    set({ playerSocket: null })
  },

  isSpectator: false,
  setIsSpectator: value => set({ isSpectator: value }),

  spectatorCount: 0,
  setSpectatorCount: count => set({ spectatorCount: count }),

  joinRoom: (roomId: string, wsUrl: string) => {
    // Close existing socket if switching rooms.
    // Reset spectator state so Game.tsx always reads fresh values for the new room.
    get().socket?.close()
    const socket = new RoomSocket(wsUrl)
    socket.connect()
    set({ socket, activeRoomId: roomId, isSpectator: false, spectatorCount: 0, presenceMap: {} })
  },
  leaveRoom: () => {
    get().socket?.close()
    set({
      socket: null,
      activeRoomId: null,
      isSpectator: false,
      spectatorCount: 0,
      presenceMap: {},
    })
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
  },

  updateSetting: (key, value) => {
    set(state => ({
      settings: { ...state.settings, [key]: value },
    }))
    if (key === 'theme') applySkin(value as SkinId)
    if (key === 'font_size') applyFontSize(value as FontSize)
  },

  setSettings: settings => set({ settings }),
}))
