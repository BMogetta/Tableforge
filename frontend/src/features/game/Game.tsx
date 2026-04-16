import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useMutation } from '@tanstack/react-query'
import { useAppStore } from '@/stores/store'
import { sessions } from '@/lib/api/sessions'
import { queue } from '@/features/lobby/api'
import { catchToAppError, type AppError } from '@/utils/errors'
import { testAttr } from '@/utils/testId'
import { useToast } from '@/ui/Toast'
import { ErrorMessage } from '@/ui/ErrorMessage'
import { sfx } from '@/lib/sfx'
import styles from './Game.module.css'
import { SurrenderModal } from './components/SurrenderModal'
import { GameTopBar } from './components/GameTopBar'
import { GameTopBarSlotContext } from './top-bar-slot'
import { PauseVoteOverlay } from './components/PauseVoteOverlay'
import { SuspendedScreen } from './components/SuspendedScreen'
import { GameOverActions } from './components/GameOverActions'
import { useNavigate, useBlocker } from '@tanstack/react-router'
import { GAME_RENDERERS } from '@/games/registry'
import { useGameSession } from './hooks/useGameSession'
import { useGameSocket } from './hooks/useGameSocket'
import { usePauseResume } from './hooks/usePauseResume'
import { useRematch } from './hooks/useRematch'
import { requestPermission, acquireWakeLock, releaseWakeLock } from '@/lib/turn-notifier'

export function Game({ sessionId }: { sessionId: string }) {
  const { t } = useTranslation()
  const player = useAppStore(s => s.player)!
  const socket = useAppStore(s => s.socket)
  const playerSocket = useAppStore(s => s.playerSocket)
  const leaveRoom = useAppStore(s => s.leaveRoom)
  const setQueued = useAppStore(s => s.setQueued)
  const isSpectator = useAppStore(s => s.isSpectator)
  const socketStatus = useAppStore(s => s.roomSocketStatus)

  const navigate = useNavigate()
  const toast = useToast()

  const [gameOver, setGameOver] = useState<{
    isOver: boolean
    winnerId: string | null
    isDraw: boolean
  }>({ isOver: false, winnerId: null, isDraw: false })

  const [showSurrenderModal, setShowSurrenderModal] = useState(false)
  const [moveError, setMoveError] = useState<AppError | null>(null)
  const [topBarSlot, setTopBarSlot] = useState<HTMLElement | null>(null)

  // Block navigation (back button, logo click) during active game.
  // Shows surrender modal instead of navigating away.
  const isActiveGame = !gameOver.isOver && !isSpectator
  const blocker = useBlocker({
    shouldBlockFn: () => isActiveGame,
    withResolver: true,
    enableBeforeUnload: () => isActiveGame,
  })

  // Reset all local state when sessionId changes (rematch creates a new session).
  useEffect(() => {
    setGameOver({ isOver: false, winnerId: null, isDraw: false })
    setShowSurrenderModal(false)
    setMoveError(null)
  }, [])

  // Preload game sounds + request notification permission + WakeLock.
  useEffect(() => {
    sfx.preload(
      'card.play',
      'card.draw',
      'card.deal',
      'chip.place',
      'game.my_turn',
      'game.round_end',
      'game.win',
      'game.lose',
      'game.draw',
      'game.start',
      'game.elimination',
    )
    if (isSpectator) return
    sfx.play('game.start')
    requestPermission()
    acquireWakeLock()
    return () => {
      releaseWakeLock()
    }
  }, [isSpectator])

  function handleGameOver(winnerId: string | null, isDraw: boolean) {
    setGameOver({ isOver: true, winnerId, isDraw })
    if (isDraw) sfx.play('game.draw')
    else if (winnerId === player.id) sfx.play('game.win')
    else sfx.play('game.lose')
  }

  // --- Hooks -----------------------------------------------------------------

  const { session, gameData, roomPlayers, loadError } = useGameSession({
    sessionId,
    onGameOver: handleGameOver,
  })

  const pauseResume = usePauseResume({ sessionId })
  const rematch = useRematch({ sessionId })

  // Sync suspended state from initial fetch — covers the "return tomorrow" case.
  useEffect(() => {
    if (session?.suspended_at) pauseResume.setSuspended(true)
  }, [session?.suspended_at, pauseResume.setSuspended])

  useGameSocket({
    sessionId,
    socket,
    playerSocket,
    pauseResume,
    rematch,
    onGameOver: handleGameOver,
  })

  // --- Mutations -------------------------------------------------------------

  const move = useMutation({
    mutationFn: (payload: Record<string, unknown>) => sessions.move(sessionId, payload),
    onSuccess: () => {
      // Move accepted — state update arrives via WS (move_applied / game_over).
      setMoveError(null)
    },
    onError: e => setMoveError(catchToAppError(e)),
  })

  const surrender = useMutation({
    mutationFn: () => sessions.surrender(sessionId),
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

  const backToQueue = useMutation({
    mutationFn: () => queue.join(player.id),
    onSuccess: () => {
      setQueued(Date.now())
      leaveRoom()
      navigate({ to: '/' })
    },
    onError: e => {
      toast.showError(catchToAppError(e))
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

  function handleBackToQueue() {
    backToQueue.mutate()
  }

  // --- Loading state ---------------------------------------------------------

  if (!session || !gameData) {
    return (
      <div className={styles.centered}>
        {loadError ? (
          <ErrorMessage error={catchToAppError(loadError)} />
        ) : (
          <p className='pulse' style={{ color: 'var(--text-muted)', letterSpacing: '0.1em' }}>
            {t('game.loading')}
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
  const canResume =
    !isSpectator && pauseResume.isSuspended && !pauseResume.votedResume && !gameOver.isOver

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

  // --- Render ----------------------------------------------------------------

  return (
    <div className={`${styles.root} page-enter`} {...testAttr('socket-status', socketStatus)}>
      <div className={styles.panel}>
        <GameTopBar
          gameId={session.game_id}
          moveCount={session.move_count}
          turnTimeoutSecs={session.turn_timeout_secs}
          lastMoveAt={session.last_move_at ?? session.started_at ?? ''}
          statusText={statusText}
          isOver={gameOver.isOver}
          isSuspended={pauseResume.isSuspended}
          canPause={canPause}
          isPausePending={pauseResume.isPausePending}
          slotRef={setTopBarSlot}
          onLobby={() =>
            gameOver.isOver || isSpectator ? handleBackToLobby() : setShowSurrenderModal(true)
          }
          onPause={pauseResume.votePause}
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
              <GameTopBarSlotContext.Provider value={topBarSlot}>
                <Renderer
                  gameId={session.game_id}
                  gameData={gameData}
                  localPlayerId={player.id}
                  onMove={payload => move.mutate(payload)}
                  disabled={isSpectator || !isMyTurn || move.isPending}
                  isOver={gameOver.isOver}
                  players={roomPlayers}
                />
              </GameTopBarSlotContext.Provider>
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
            isRanked={session.mode === 'ranked'}
            votedRematch={rematch.votedRematch}
            rematchVotes={rematch.rematchVotes}
            totalPlayers={rematch.totalPlayers}
            isRematchPending={rematch.isPending}
            isBackToQueuePending={backToQueue.isPending}
            onBackToLobby={handleBackToLobby}
            onViewReplay={handleViewReplay}
            onRematch={rematch.voteRematch}
            onBackToQueue={handleBackToQueue}
          />
        )}
      </div>

      {(showSurrenderModal || blocker.status === 'blocked') && (
        <SurrenderModal
          onConfirm={() => surrender.mutate()}
          onCancel={() => {
            setShowSurrenderModal(false)
            if (blocker.status === 'blocked') blocker.reset()
          }}
          isPending={surrender.isPending}
        />
      )}
    </div>
  )
}
