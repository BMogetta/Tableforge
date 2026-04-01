import { useEffect, useState } from 'react'
import { useMutation, useQueryClient } from '@tanstack/react-query'
import { useAppStore } from '@/stores/store'
import { sessions } from '@/lib/api/sessions'
import { catchToAppError, type AppError } from '@/utils/errors'
import { useToast } from '@/ui/Toast'
import { ErrorMessage } from '@/ui/ErrorMessage'
import { keys } from '@/lib/queryClient'
import styles from './Game.module.css'
import { SurrenderModal } from './components/SurrenderModal'
import { GameHeader } from './components/GameHeader'
import { GameStatus } from './components/GameStatus'
import { PauseVoteOverlay } from './components/PauseVoteOverlay'
import { SuspendedScreen } from './components/SuspendedScreen'
import { GameOverActions } from './components/GameOverActions'
import { useNavigate } from '@tanstack/react-router'
import { GAME_RENDERERS } from '@/games/registry'
import { useGameSession } from './hooks/useGameSession'
import { useGameSocket } from './hooks/useGameSocket'
import { usePauseResume } from './hooks/usePauseResume'
import { useRematch } from './hooks/useRematch'
import { SessionCache } from './api/session-cache'
import { ResultStatus } from '@/lib/api-generated'

export function Game({ sessionId }: { sessionId: string }) {
  const player = useAppStore(s => s.player)!
  const socket = useAppStore(s => s.socket)
  const playerSocket = useAppStore(s => s.playerSocket)
  const leaveRoom = useAppStore(s => s.leaveRoom)
  const isSpectator = useAppStore(s => s.isSpectator)
  const presenceMap = useAppStore(s => s.presenceMap)
  const setPlayerPresence = useAppStore(s => s.setPlayerPresence)
  // Game.tsx — initialize socket status from the store socket state.
  // The socket may already be connected when Game mounts (navigated from Room),
  // so we cannot rely on catching ws_connected after mount.
  const [socketStatus, setSocketStatus] = useState<'connecting' | 'connected' | 'disconnected'>(
    () => (socket ? 'connected' : 'connecting'),
  )

  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()

  const [gameOver, setGameOver] = useState<{
    isOver: boolean
    winnerId: string | null
    isDraw: boolean
  }>({ isOver: false, winnerId: null, isDraw: false })

  const [showSurrenderModal, setShowSurrenderModal] = useState(false)
  const [moveError, setMoveError] = useState<AppError | null>(null)

  // Reset all local state when sessionId changes (rematch creates a new session).
  useEffect(() => {
    setGameOver({ isOver: false, winnerId: null, isDraw: false })
    setShowSurrenderModal(false)
    setMoveError(null)
  }, [sessionId])

  function handleGameOver(winnerId: string | null, isDraw: boolean) {
    setGameOver({ isOver: true, winnerId, isDraw })
  }

  useEffect(() => {
    if (!socket) return
    const off = socket.on(event => {
      if (event.type === 'ws_connected') setSocketStatus('connected')
      if (event.type === 'ws_disconnected') setSocketStatus('disconnected')
    })
    return () => off()
  }, [socket])

  // --- Hooks -----------------------------------------------------------------

  const { session, gameData, roomPlayers, loadError } = useGameSession({
    sessionId,
    onGameOver: handleGameOver,
  })

  const pauseResume = usePauseResume({ sessionId, playerId: player.id })
  const rematch = useRematch({ sessionId, playerId: player.id })

  // Sync suspended state from initial fetch — covers the "return tomorrow" case.
  useEffect(() => {
    if (session?.suspended_at) pauseResume.setSuspended(true)
  }, [session?.id, session?.suspended_at])

  useGameSocket({
    sessionId,
    socket,
    playerSocket,
    pauseResume,
    rematch,
    onGameOver: handleGameOver,
    onPresenceUpdate: setPlayerPresence,
  })

  // --- Mutations -------------------------------------------------------------

  const move = useMutation({
    mutationFn: (payload: Record<string, unknown>) => sessions.move(sessionId, player.id, payload),
    onSuccess: res => {
      setMoveError(null)
      qc.setQueryData(keys.session(sessionId), {
        session: res.session,
        state: res.state,
        // Normalize MoveResponse.result (Result shape) to SessionCache.result shape.
        result: res.result
          ? {
              winner_id: res.result.winner_id ?? null,
              is_draw: res.result.status === ResultStatus.ResultDraw,
            }
          : null,
      } satisfies SessionCache)
      if (res.is_over) {
        setGameOver({
          isOver: true,
          winnerId: res.result?.winner_id ?? null,
          isDraw: res.result?.status === ResultStatus.ResultDraw,
        })
      }
    },
    onError: e => setMoveError(catchToAppError(e)),
  })

  const surrender = useMutation({
    mutationFn: () => sessions.surrender(sessionId, player.id),
    onSuccess: () => {
      setShowSurrenderModal(false)
      leaveRoom()
      navigate({ to: '/' })
    },
    onError: e => {
      toast.showError(catchToAppError(e))
      setShowSurrenderModal(false)
    },
  })

  // --- Handlers --------------------------------------------------------------

  function handleBackToLobby() {
    leaveRoom()
    navigate({ to: '/' })
  }

  function handleViewReplay() {
    leaveRoom()
    navigate({ to: '/sessions/$sessionId/history', params: { sessionId } })
  }

  // --- Loading state ---------------------------------------------------------

  if (!session || !gameData) {
    return (
      <div className={styles.centered}>
        {loadError ? (
          <ErrorMessage error={catchToAppError(loadError)} />
        ) : (
          <p className='pulse' style={{ color: 'var(--text-muted)', letterSpacing: '0.1em' }}>
            Loading game...
          </p>
        )}
      </div>
    )
  }

  // --- Derived state ---------------------------------------------------------

  const isMyTurn =
    !isSpectator &&
    gameData.current_player_id === player.id &&
    !gameOver.isOver &&
    !session.finished_at &&
    !pauseResume.isSuspended

  const canPause =
    !isSpectator && !gameOver.isOver && !pauseResume.isSuspended && !pauseResume.votedPause
  const canResume = !isSpectator && pauseResume.isSuspended && !pauseResume.votedResume && !gameOver.isOver

  let statusText = ''
  if (isSpectator) {
    if (pauseResume.isSuspended) statusText = 'Game paused'
    else if (gameOver.winnerId) statusText = 'Game over'
    else if (gameOver.isDraw) statusText = 'Draw!'
    else statusText = 'Watching...'
  } else if (pauseResume.isSuspended) {
    statusText = 'Game paused'
  } else if (gameOver.winnerId) {
    statusText = gameOver.winnerId === player.id ? 'You won!' : 'You lost.'
  } else if (gameOver.isDraw) {
    statusText = 'Draw!'
  } else if (isMyTurn) {
    statusText = 'Your turn'
  } else {
    statusText = "Opponent's turn"
  }

  const Renderer = GAME_RENDERERS[session.game_id]
  const opponentId = roomPlayers.find(p => p.id !== player.id)?.id ?? null
  const opponentOnline = opponentId ? (presenceMap[opponentId] ?? false) : false

  // --- Render ----------------------------------------------------------------

  return (
    <div
      className={`${styles.root} page-enter`}
      {...(import.meta.env.VITE_TEST_MODE === 'true' && { 'data-socket-status': socketStatus })}
    >
      <div className={styles.panel}>
        <GameHeader
          gameId={session.game_id}
          moveCount={session.move_count}
          turnTimeoutSecs={session.turn_timeout_secs}
          lastMoveAt={session.last_move_at ?? session.started_at ?? ''}
          isSuspended={pauseResume.isSuspended}
          canPause={canPause}
          isPausePending={pauseResume.isPausePending}
          isOver={gameOver.isOver}
          isSpectator={isSpectator}
          onLobby={() =>
            gameOver.isOver || isSpectator ? handleBackToLobby() : setShowSurrenderModal(true)
          }
          onPause={pauseResume.votePause}
        />

        <GameStatus
          statusText={statusText}
          isMyTurn={isMyTurn}
          isOver={gameOver.isOver}
          isSuspended={pauseResume.isSuspended}
          isSpectator={isSpectator}
          winnerId={gameOver.winnerId}
          playerId={player.id}
          opponentId={opponentId}
          opponentOnline={opponentOnline}
        />

        {!pauseResume.isSuspended && pauseResume.pauseVotes.length > 0 && (
          <PauseVoteOverlay
            votes={pauseResume.pauseVotes}
            required={pauseResume.pauseRequired}
            votedPause={pauseResume.votedPause}
            isPending={pauseResume.isPausePending}
            isSpectator={isSpectator}
            onVote={pauseResume.votePause}
          />
        )}

        {pauseResume.isSuspended && !gameOver.isOver ? (
          <SuspendedScreen
            resumeVotes={pauseResume.resumeVotes}
            resumeRequired={pauseResume.resumeRequired}
            votedResume={pauseResume.votedResume}
            isPending={pauseResume.isResumePending}
            canResume={canResume}
            onResume={pauseResume.voteResume}
            onBackToLobby={handleBackToLobby}
          />
        ) : (
          <div className={styles.boardWrapper}>
            {Renderer ? (
              <Renderer
                gameId={session.game_id}
                gameData={gameData}
                localPlayerId={player.id}
                onMove={payload => move.mutate(payload)}
                disabled={isSpectator || !isMyTurn || move.isPending}
                isOver={gameOver.isOver}
                players={roomPlayers}
              />
            ) : (
              <p style={{ color: 'var(--text-muted)', fontSize: 13 }}>
                No renderer available for game: {session.game_id}
              </p>
            )}
          </div>
        )}

        {moveError && <ErrorMessage error={moveError} />}

        {gameOver.isOver && (
          <GameOverActions
            isSpectator={isSpectator}
            votedRematch={rematch.votedRematch}
            rematchVotes={rematch.rematchVotes}
            totalPlayers={rematch.totalPlayers}
            isRematchPending={rematch.isPending}
            onBackToLobby={handleBackToLobby}
            onViewReplay={handleViewReplay}
            onRematch={rematch.voteRematch}
          />
        )}
      </div>

      {showSurrenderModal && (
        <SurrenderModal
          onConfirm={() => surrender.mutate()}
          onCancel={() => setShowSurrenderModal(false)}
          isPending={surrender.isPending}
        />
      )}
    </div>
  )
}
