import { useEffect } from 'react'
import { useQueryClient } from '@tanstack/react-query'
import { keys } from '@/lib/queryClient'
import type { RoomSocket, PlayerSocket, WsPayloadMoveResult } from '@/lib/ws'
import type { PauseResumeState } from './usePauseResume'
import type { RematchState } from './useRematch'
import { ResultStatus } from '@/lib/api'
import { notifyTurn } from '@/lib/turn-notifier'
import { sfx } from '@/lib/sfx'
import { useAppStore } from '@/stores/store'
import type { SessionCache } from '../api/session-cache'

interface UseGameSocketOptions {
  sessionId: string
  socket: RoomSocket | null
  playerSocket: PlayerSocket | null
  pauseResume: Pick<
    PauseResumeState,
    'setPauseVoteUpdate' | 'setResumeVoteUpdate' | 'onSessionSuspended' | 'onSessionResumed'
  >
  rematch: Pick<RematchState, 'onRematchVote' | 'onRematchReady' | 'onRematchGameStarted'>
  onGameOver: (winnerId: string | null, isDraw: boolean) => void
  onPresenceUpdate: (playerId: string, online: boolean) => void
}

/**
 * Subscribes to room and player WebSocket events for the active game session.
 *
 * Responsibilities:
 * - Handles move_applied and game_over on both room socket and player socket.
 * - Deduplicates WS updates against the React Query cache by checking move_count
 *   — prevents applying stale or out-of-order events.
 * - Delegates pause/resume and rematch event handling to the respective hooks
 *   via the pauseResume and rematch callbacks.
 * - Invalidates the session query on ws_connected to recover from reconnections.
 *
 * Separation of concerns:
 * - This hook does NOT own any state — it only reads from sockets and writes
 *   to the React Query cache or calls provided callbacks.
 * - All state mutations go through the callbacks passed in (pauseResume, rematch,
 *   onGameOver, onPresenceUpdate).
 *
 * @testability
 * Pass mock socket objects with a spy `.on()` method.
 * Trigger events by calling the listener captured from the spy.
 * Assert that queryClient cache is updated and callbacks are called.
 */
export function useGameSocket({
  sessionId,
  socket,
  playerSocket,
  pauseResume,
  rematch,
  onGameOver,
  onPresenceUpdate,
}: UseGameSocketOptions): void {
  const qc = useQueryClient()

  function handleMovePayload(
    payload: WsPayloadMoveResult,
    type: 'move_applied' | 'game_over',
  ): void {
    const current = qc.getQueryData<SessionCache>(keys.session(sessionId))

    // Deduplicate move_applied events — ignore if older than or equal to cached.
    // Never deduplicate game_over — timeouts can end a game without changing move_count.
    if (
      type === 'move_applied' &&
      current?.session?.move_count !== undefined &&
      payload.session.move_count <= current.session.move_count
    ) {
      return
    }

    if (type === 'move_applied') {
      qc.setQueryData(keys.session(sessionId), {
        session: payload.session,
        state: payload.state,
        is_over: false,
      })
      // Notify if it's now the local player's turn and the tab is hidden.
      const localId = useAppStore.getState().player?.id
      if (localId && payload.state.current_player_id === localId) {
        notifyTurn()
        sfx.play('game.my_turn')
      }
    } else {
      qc.setQueryData(keys.session(sessionId), {
        session: payload.session,
        state: payload.state,
        is_over: true,
        result: payload.result
          ? {
              winner_id: payload.result.winner_id,
              is_draw: payload.result.status === ResultStatus.Draw,
            }
          : null,
      })
      onGameOver(payload.result?.winner_id ?? null, payload.result?.status === ResultStatus.Draw)
    }
  }

  // Room socket — handles all game events for the current session.
  useEffect((): (() => void) | undefined => {
    if (!socket) return

    const off = socket.on(event => {
      if (event.type === 'ws_connected') {
        qc.invalidateQueries({ queryKey: keys.session(sessionId) })
      }
      if (event.type === 'presence_update') {
        onPresenceUpdate(event.payload.player_id, event.payload.online)
      }
      if (event.type === 'move_applied') {
        handleMovePayload(event.payload, 'move_applied')
      }
      if (event.type === 'game_over') {
        handleMovePayload(event.payload, 'game_over')
      }
      if (event.type === 'rematch_vote') {
        rematch.onRematchVote(event.payload.votes, event.payload.total_players)
      }
      if (event.type === 'rematch_ready') {
        rematch.onRematchReady(event.payload.room_id)
      }
      if (event.type === 'game_started') {
        rematch.onRematchGameStarted(event.payload.session.id)
      }
      if (event.type === 'pause_vote_update') {
        pauseResume.setPauseVoteUpdate(event.payload.votes ?? [], event.payload.required ?? 0)
      }
      if (event.type === 'session_suspended') {
        pauseResume.onSessionSuspended()
      }
      if (event.type === 'resume_vote_update') {
        pauseResume.setResumeVoteUpdate(event.payload.votes ?? [], event.payload.required ?? 0)
      }
      if (event.type === 'session_resumed') {
        pauseResume.onSessionResumed()
      }
    })

    return () => off()
  }, [socket, sessionId, handleMovePayload, onPresenceUpdate, pauseResume.onSessionResumed, pauseResume.onSessionSuspended, pauseResume.setPauseVoteUpdate, pauseResume.setResumeVoteUpdate, qc.invalidateQueries, rematch.onRematchGameStarted, rematch.onRematchReady, rematch.onRematchVote])

  // Player socket — receives move and game_over events for sessions the player
  // is part of, regardless of which room socket is active. Used to handle
  // updates when the player is navigating between pages.
  useEffect((): (() => void) | undefined => {
    if (!playerSocket) return

    const off = playerSocket.on(event => {
      if (event.type === 'move_applied') {
        handleMovePayload(event.payload, 'move_applied')
      }
      if (event.type === 'game_over') {
        handleMovePayload(event.payload, 'game_over')
      }
    })

    return () => off()
  }, [playerSocket, handleMovePayload])
}
