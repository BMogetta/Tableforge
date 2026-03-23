import { useEffect, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAppStore } from '../stores/store'
import { sessions, rooms, type GameSession } from '../lib/api'
import { catchToAppError, type AppError } from '../utils/errors'
import { useToast } from '../components/ui/Toast'
import { ErrorMessage } from '../components/ui/ErrorMessage'
import { keys } from '../lib/queryClient'
import styles from './Game.module.css'
import { SurrenderModal } from '../components/SurrenderModal'
import type { WsPayloadMoveResult } from '../lib/ws'
import { useNavigate } from '@tanstack/react-router'
import { GAME_RENDERERS, GameData } from '../games/registry'

interface MoveResult {
  session: GameSession
  state: GameData
  is_over: boolean
  result?: { winner_id?: string; is_draw?: boolean }
}

// ---------------------------------------------------------------------------
// Game page
// ---------------------------------------------------------------------------

export function Game({ sessionId }: { sessionId: string }) {
  const player = useAppStore(s => s.player)!
  const socket = useAppStore(s => s.socket)
  const playerSocket = useAppStore(s => s.playerSocket)
  const leaveRoom = useAppStore(s => s.leaveRoom)
  const isSpectator = useAppStore(s => s.isSpectator)
  const presenceMap = useAppStore(s => s.presenceMap)
  const setPlayerPresence = useAppStore(s => s.setPlayerPresence)
  const navigate = useNavigate()
  const toast = useToast()
  const qc = useQueryClient()

  const [isOver, setIsOver] = useState(false)
  const [winnerId, setWinnerId] = useState<string | null>(null)
  const [isDraw, setIsDraw] = useState(false)
  const [showSurrenderModal, setShowSurrenderModal] = useState(false)
  const [rematchVotes, setRematchVotes] = useState(0)
  const [totalPlayers, setTotalPlayers] = useState(0)
  const [votedRematch, setVotedRematch] = useState(false)
  const [moveError, setMoveError] = useState<AppError | null>(null)

  // Pause / resume state
  const [isSuspended, setIsSuspended] = useState(false)
  const [pauseVotes, setPauseVotes] = useState<string[]>([])
  const [pauseRequired, setPauseRequired] = useState(0)
  const [resumeVotes, setResumeVotes] = useState<string[]>([])
  const [resumeRequired, setResumeRequired] = useState(0)
  const [votedPause, setVotedPause] = useState(false)
  const [votedResume, setVotedResume] = useState(false)

  useEffect(() => {
    setIsOver(false)
    setWinnerId(null)
    setIsDraw(false)
    setVotedRematch(false)
    setRematchVotes(0)
    setTotalPlayers(0)
    setShowSurrenderModal(false)
    setMoveError(null)
    setIsSuspended(false)
    setPauseVotes([])
    setPauseRequired(0)
    setResumeVotes([])
    setResumeRequired(0)
    setVotedPause(false)
    setVotedResume(false)
  }, [sessionId])

  const { data, error: loadError } = useQuery({
    queryKey: keys.session(sessionId!),
    queryFn: () => sessions.get(sessionId!),
    refetchOnWindowFocus: false,
    staleTime: 0,
    refetchInterval: query => {
      const d = query.state.data as { session?: { finished_at?: string } } | undefined
      return d?.session?.finished_at ? false : 3_000
    },
  })

  const session = data?.session ?? null
  const gameData = data?.state ?? null

  // Sync suspended state from initial fetch — covers the "return tomorrow" case
  // where the player navigates back to an already-suspended session.
  useEffect(() => {
    if (!session) return
    if (session.suspended_at) {
      setIsSuspended(true)
    }
  }, [session?.id, session?.suspended_at])

  const { data: roomData } = useQuery({
    queryKey: keys.room(session?.room_id ?? ''),
    queryFn: () => rooms.get(session!.room_id),
    enabled: !!session?.room_id,
    staleTime: Infinity,
  })

  const roomPlayers =
    roomData?.players.map(p => ({
      id: p.id,
      username: p.username,
    })) ?? []

  useEffect(() => {
    if (!data?.session?.finished_at) return
    setIsOver(true)
    const result = data.result ?? null
    if (result?.winner_id) setWinnerId(result.winner_id)
    else if (result?.is_draw) setIsDraw(true)
  }, [data?.session?.finished_at, (data as any)?.result])

  function handleMovePayload(payload: WsPayloadMoveResult, type: 'move_applied' | 'game_over') {
    const current = qc.getQueryData<MoveResult>(keys.session(sessionId!))
    if (
      current?.session?.move_count !== undefined &&
      payload.session.move_count <= current.session.move_count
    ) {
      return
    }
    if (type === 'move_applied') {
      qc.setQueryData(keys.session(sessionId!), {
        session: payload.session,
        state: payload.state,
        is_over: false,
      })
    } else {
      qc.setQueryData(keys.session(sessionId!), {
        session: payload.session,
        state: payload.state,
        is_over: true,
      })
      setIsOver(true)
      if (payload.result?.winner_id) setWinnerId(payload.result.winner_id)
      else if (payload.result?.is_draw) setIsDraw(true)
    }
  }

  useEffect((): (() => void) | void => {
    if (!socket) return
    const off = socket.on(event => {
      if (event.type === 'ws_connected') {
        qc.invalidateQueries({ queryKey: keys.session(sessionId!) })
      }
      if (event.type === 'presence_update') {
        setPlayerPresence(event.payload.player_id, event.payload.online)
      }
      if (event.type === 'move_applied') {
        handleMovePayload(event.payload, 'move_applied')
      }
      if (event.type === 'game_over') {
        handleMovePayload(event.payload, 'game_over')
      }
      if (event.type === 'rematch_vote') {
        setRematchVotes(event.payload.votes)
        setTotalPlayers(event.payload.total_players)
      }
      if (event.type === 'rematch_ready') {
        navigate({
          to: '/rooms/$roomId',
          params: { roomId: event.payload.room_id },
        })
      }
      if (event.type === 'pause_vote_update') {
        setPauseVotes(event.payload.votes ?? [])
        setPauseRequired(event.payload.required ?? 0)
      }
      if (event.type === 'session_suspended') {
        setIsSuspended(true)
        setPauseVotes([])
      }
      if (event.type === 'resume_vote_update') {
        setResumeVotes(event.payload.votes ?? [])
        setResumeRequired(event.payload.required ?? 0)
      }
      if (event.type === 'session_resumed') {
        setIsSuspended(false)
        setResumeVotes([])
        setVotedPause(false)
        setVotedResume(false)
      }
    })
    return () => off()
  }, [socket, sessionId, qc, navigate])

  useEffect((): (() => void) | void => {
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
  }, [playerSocket, sessionId, qc])

  const move = useMutation({
    mutationFn: (payload: Record<string, unknown>) => sessions.move(sessionId!, player.id, payload),
    onSuccess: res => {
      setMoveError(null)
      qc.setQueryData(keys.session(sessionId!), {
        session: res.session,
        state: res.state,
        is_over: res.is_over,
      })
      if (res.is_over) setIsOver(true)
    },
    onError: e => {
      setMoveError(catchToAppError(e))
    },
  })

  const surrender = useMutation({
    mutationFn: () => sessions.surrender(sessionId!, player.id),
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

  const rematch = useMutation({
    mutationFn: () => sessions.rematch(sessionId!, player.id),
    onSuccess: res => {
      setVotedRematch(true)
      setRematchVotes(res.votes)
      setTotalPlayers(res.total_players)
    },
    onError: e => {
      toast.showError(catchToAppError(e))
    },
  })

  const pauseMutation = useMutation({
    mutationFn: () => sessions.pause(sessionId!, player.id),
    onSuccess: res => {
      setVotedPause(true)
      setPauseVotes(res.votes)
      setPauseRequired(res.required)
    },
    onError: e => {
      toast.showError(catchToAppError(e))
    },
  })

  const resumeMutation = useMutation({
    mutationFn: () => sessions.resume(sessionId!, player.id),
    onSuccess: res => {
      setVotedResume(true)
      setResumeVotes(res.votes)
      setResumeRequired(res.required)
    },
    onError: e => {
      toast.showError(catchToAppError(e))
    },
  })

  function handleBackToLobby() {
    leaveRoom()
    navigate({ to: '/' })
  }

  function handleViewReplay() {
    leaveRoom()
    navigate({
      to: '/sessions/$sessionId/history',
      params: { sessionId },
    })
  }

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

  const isMyTurn =
    !isSpectator &&
    gameData.current_player_id === player.id &&
    !isOver &&
    !session.finished_at &&
    !isSuspended
  const canPause = !isSpectator && !isOver && !isSuspended && !votedPause
  const canResume = !isSpectator && isSuspended && !votedResume

  let statusText = ''
  if (isSpectator) {
    if (isSuspended) statusText = 'Game paused'
    else if (winnerId) statusText = 'Game over'
    else if (isDraw) statusText = 'Draw!'
    else statusText = 'Watching...'
  } else if (isSuspended) {
    statusText = 'Game paused'
  } else if (winnerId) {
    statusText = winnerId === player.id ? 'You won!' : 'You lost.'
  } else if (isDraw) {
    statusText = 'Draw!'
  } else if (isMyTurn) {
    statusText = 'Your turn'
  } else {
    statusText = "Opponent's turn"
  }

  const Renderer = GAME_RENDERERS[session.game_id]

  return (
    <div className={`${styles.root} page-enter`}>
      <div className={styles.panel}>
        <header className={styles.header}>
          <button
            className='btn btn-ghost'
            onClick={() =>
              isOver || isSpectator ? handleBackToLobby() : setShowSurrenderModal(true)
            }
            style={{ padding: '4px 10px', fontSize: 11 }}
          >
            ← Lobby
          </button>
          <div className={styles.gameInfo}>
            <span className={styles.gameId}>{session.game_id}</span>
            <span className={styles.moveCount}>Move {session.move_count}</span>
          </div>
          {canPause && (
            <button
              data-testid='pause-btn'
              className='btn btn-ghost'
              onClick={() => pauseMutation.mutate()}
              disabled={pauseMutation.isPending}
              style={{ padding: '4px 10px', fontSize: 11 }}
            >
              ⏸ Pause
            </button>
          )}
        </header>

        <div className={styles.status}>
          <div
            className={`${styles.statusDot} ${isMyTurn ? styles.dotActive : ''} ${isOver || isSuspended ? styles.dotOver : ''}`}
          />
          <span
            data-testid='game-status'
            className={`${styles.statusText} ${winnerId === player.id ? styles.win : ''} ${winnerId && winnerId !== player.id ? styles.lose : ''} ${isSuspended ? styles.suspended : ''}`}
          >
            {statusText}
          </span>
          {!isSpectator &&
            !isSuspended &&
            (() => {
              const opponentId = roomPlayers.find(p => p.id !== player.id)?.id ?? null
              if (!opponentId) return null
              const opponentOnline = presenceMap[opponentId] ?? false
              return (
                <span className={styles.opponentPresence} data-testid='opponent-presence'>
                  <span
                    className={styles.presenceDot}
                    data-online={String(opponentOnline)}
                    data-testid='opponent-presence-dot'
                  />
                  <span data-testid='opponent-presence-text'>
                    {opponentOnline ? 'Opponent online' : 'Opponent offline'}
                  </span>
                </span>
              )
            })()}
        </div>

        {/* Pause vote overlay — shown while a pause vote is in progress */}
        {!isSuspended && pauseVotes.length > 0 && (
          <div className={styles.voteOverlay} data-testid='pause-vote-overlay'>
            <p className={styles.voteTitle}>Pause requested</p>
            <p className={styles.voteCount}>
              {pauseVotes.length} / {pauseRequired} voted
            </p>
            {!isSpectator && !votedPause && (
              <button
                data-testid='vote-pause-btn'
                className='btn btn-ghost'
                onClick={() => pauseMutation.mutate()}
                disabled={pauseMutation.isPending}
              >
                Vote to Pause
              </button>
            )}
            {votedPause && <p className={styles.voteWaiting}>Waiting for opponent…</p>}
          </div>
        )}

        {/* Suspended screen — replaces the board when game is paused */}
        {isSuspended ? (
          <div className={styles.suspendedScreen} data-testid='suspended-screen'>
            <span className={styles.suspendedIcon}>⏸</span>
            <p className={styles.suspendedTitle}>Game Paused</p>
            <p className={styles.suspendedBody}>All players must vote to resume the game.</p>
            {resumeVotes.length > 0 && (
              <p className={styles.voteCount} data-testid='resume-vote-count'>
                {resumeVotes.length} / {resumeRequired} voted to resume
              </p>
            )}
            {canResume && (
              <button
                data-testid='vote-resume-btn'
                className='btn btn-primary'
                onClick={() => resumeMutation.mutate()}
                disabled={resumeMutation.isPending}
              >
                Vote to Resume
              </button>
            )}
            {votedResume && <p className={styles.voteWaiting}>Waiting for opponent…</p>}
            <button className='btn btn-ghost' onClick={handleBackToLobby} style={{ marginTop: 8 }}>
              ← Back to Lobby
            </button>
          </div>
        ) : (
          <div className={styles.boardWrapper}>
            {Renderer ? (
              <Renderer
                gameId={session.game_id}
                gameData={gameData}
                localPlayerId={player.id}
                onMove={payload => move.mutate(payload)}
                disabled={isSpectator || !isMyTurn || move.isPending}
                isOver={isOver}
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

        {isOver && (
          <div className={styles.gameOver}>
            <button className='btn btn-ghost' onClick={handleBackToLobby}>
              Back to Lobby
            </button>
            <button
              className='btn btn-ghost'
              data-testid='view-replay-btn'
              onClick={handleViewReplay}
            >
              View Replay
            </button>
            {!isSpectator && (
              <button
                className='btn btn-primary'
                data-testid='rematch-btn'
                onClick={() => rematch.mutate()}
                disabled={votedRematch || rematch.isPending}
              >
                {votedRematch
                  ? `Waiting for opponent… (${rematchVotes}/${totalPlayers})`
                  : rematchVotes > 0
                    ? `Rematch (${rematchVotes}/${totalPlayers} voted)`
                    : 'Rematch'}
              </button>
            )}
          </div>
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
