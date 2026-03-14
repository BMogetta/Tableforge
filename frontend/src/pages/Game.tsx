import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAppStore } from '../store'
import { sessions, rooms, type GameSession } from '../api'
import { keys } from '../queryClient'
import TicTacToeBoard, { type TicTacToeState } from '../components/TicTacToe'
import LoveLetter, { type LoveLetterState } from '../components/loveletter/LoveLetter'
import styles from './Game.module.css'
import SurrenderModal from '../components/SurrenderModal'
import type { WsPayloadMoveResult } from '../ws'

interface GameData {
  current_player_id: string
  data: unknown
}

interface MoveResult {
  session: GameSession
  state: GameData
  is_over: boolean
  result?: { winner_id?: string; is_draw?: boolean }
}

// ---------------------------------------------------------------------------
// Game renderer registry — add new games here.
// ---------------------------------------------------------------------------

interface RendererProps {
  gameId: string
  gameData: GameData
  localPlayerId: string
  onMove: (payload: Record<string, unknown>) => void
  disabled: boolean
  isOver: boolean
  players: { id: string; username: string }[]
}

type RendererComponent = React.FC<RendererProps>

const TicTacToeRenderer: RendererComponent = ({ gameData, localPlayerId, onMove, disabled }) => (
  <TicTacToeBoard
    state={gameData.data as TicTacToeState}
    currentPlayerId={gameData.current_player_id}
    localPlayerId={localPlayerId}
    onMove={(cell) => onMove({ cell })}
    disabled={disabled}
  />
)

const LoveLetterRenderer: RendererComponent = ({ gameData, localPlayerId, onMove, disabled, isOver, players }) => (
  <LoveLetter
    state={gameData.data as LoveLetterState}
    currentPlayerId={gameData.current_player_id}
    localPlayerId={localPlayerId}
    onMove={onMove}
    disabled={disabled}
    isOver={isOver}
    players={players}
  />
)

const GAME_RENDERERS: Record<string, RendererComponent> = {
  tictactoe: TicTacToeRenderer,
  loveletter: LoveLetterRenderer,
}

// ---------------------------------------------------------------------------
// Game page
// ---------------------------------------------------------------------------

export default function Game() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const player = useAppStore((s) => s.player)!
  const socket = useAppStore((s) => s.socket)
  const playerSocket = useAppStore((s) => s.playerSocket)
  const leaveRoom = useAppStore((s) => s.leaveRoom)
  const isSpectator = useAppStore((s) => s.isSpectator)
  const spectatorCount = useAppStore((s) => s.spectatorCount)
  const presenceMap = useAppStore((s) => s.presenceMap)
  const setPlayerPresence = useAppStore((s) => s.setPlayerPresence)
  const navigate = useNavigate()
  const qc = useQueryClient()

  const [isOver, setIsOver] = useState(false)
  const [winnerId, setWinnerId] = useState<string | null>(null)
  const [isDraw, setIsDraw] = useState(false)
  const [showSurrenderModal, setShowSurrenderModal] = useState(false)
  const [rematchVotes, setRematchVotes] = useState(0)
  const [totalPlayers, setTotalPlayers] = useState(0)
  const [votedRematch, setVotedRematch] = useState(false)

  useEffect(() => {
    setIsOver(false)
    setWinnerId(null)
    setIsDraw(false)
    setVotedRematch(false)
    setRematchVotes(0)
    setTotalPlayers(0)
    setShowSurrenderModal(false)
  }, [sessionId])

  // Fetch session state on mount. Poll every 3s as a fallback for missed WS events.
  // Polling stops once finished_at is set.
  const { data, error: loadError } = useQuery({
    queryKey: keys.session(sessionId!),
    queryFn: () => sessions.get(sessionId!),
    refetchOnWindowFocus: false,
    staleTime: 0,
    refetchInterval: (query) => {
      const d = query.state.data as { session?: { finished_at?: string } } | undefined
      return d?.session?.finished_at ? false : 3_000
    },
  })

  const session = data?.session ?? null
  const gameData = data?.state ?? null

  // Fetch room players for games that need usernames (e.g. Love Letter).
  // Skipped until session is loaded so we have the room_id.
  const { data: roomData } = useQuery({
    queryKey: keys.room(session?.room_id ?? ''),
    queryFn: () => rooms.get(session!.room_id),
    enabled: !!session?.room_id,
    staleTime: Infinity, // players don't change during a session
  })

  const roomPlayers = roomData?.players.map((p) => ({
    id: p.id,
    username: p.username,
  })) ?? []

  // Sync game-over state from fetch (covers page refresh and missed WS events).
  useEffect(() => {
    if (!data?.session?.finished_at) return
    setIsOver(true)
    const result = data.result ?? null
    if (result?.winner_id) setWinnerId(result.winner_id)
    else if (result?.is_draw) setIsDraw(true)
  }, [data?.session?.finished_at, (data as any)?.result])

  // Shared handler for move_applied / game_over payloads.
  // Deduplicates by move_count to handle delivery from both room and player
  // channels (Redis mode delivers filtered state via player channel for games
  // with private state such as Love Letter).
  function handleMovePayload(payload: WsPayloadMoveResult, type: 'move_applied' | 'game_over') {
    const current = qc.getQueryData<MoveResult>(keys.session(sessionId!))
    if (
      current?.session?.move_count !== undefined &&
      payload.session.move_count <= current.session.move_count
    ) {
      return // already processed
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

  // Room socket — all games, all events except filtered move events in Redis mode.
  useEffect((): (() => void) | void => {
    if (!socket) return
    const off = socket.on((event) => {
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
        navigate(`/rooms/${event.payload.room_id}`)
      }
    })
    return () => off()
  }, [socket, sessionId, qc, navigate])

  // Player socket — receives filtered move_applied / game_over for games with
  // private state (Love Letter) in Redis multi-instance mode. The room socket
  // handles the same events in single-instance mode, so deduplication by
  // move_count ensures no double processing.
  useEffect((): (() => void) | void => {
    if (!playerSocket) return
    const off = playerSocket.on((event) => {
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
    mutationFn: (payload: Record<string, unknown>) =>
      sessions.move(sessionId!, player.id, payload),
    onSuccess: (res) => {
      qc.setQueryData(keys.session(sessionId!), {
        session: res.session,
        state: res.state,
        is_over: res.is_over,
      })
      if (res.is_over) setIsOver(true)
    },
  })

  const surrender = useMutation({
    mutationFn: () => sessions.surrender(sessionId!, player.id),
    onSuccess: () => {
      // The game_over WS event will update state. Close modal and leave.
      setShowSurrenderModal(false)
      leaveRoom()
      navigate('/')
    },
  })

  const rematch = useMutation({
    mutationFn: () => sessions.rematch(sessionId!, player.id),
    onSuccess: (res) => {
      setVotedRematch(true)
      setRematchVotes(res.votes)
      setTotalPlayers(res.total_players)
      // When all players have voted, the rematch_ready WS event handles navigation.
    },
  })

  function handleBackToLobby() {
    leaveRoom()
    navigate('/')
  }

  function handleViewReplay() {
    leaveRoom()
    navigate(`/sessions/${sessionId}/history`)
  }

  if (!session || !gameData) {
    return (
      <div className={styles.centered}>
        {loadError
          ? <p style={{ color: 'var(--danger)', fontSize: 13 }}>Failed to load game.</p>
          : <p className="pulse" style={{ color: 'var(--text-muted)', letterSpacing: '0.1em' }}>Loading game...</p>
        }
      </div>
    )
  }

  const isMyTurn = !isSpectator && gameData.current_player_id === player.id && !isOver

  let statusText = ''
  if (isSpectator) {
    if (winnerId) statusText = 'Game over'
    else if (isDraw) statusText = 'Draw!'
    else statusText = 'Watching...'
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
            className="btn btn-ghost"
            onClick={() => isOver || isSpectator ? handleBackToLobby() : setShowSurrenderModal(true)}
            style={{ padding: '4px 10px', fontSize: 11 }}
          >
            ← Lobby
          </button>
          <div className={styles.gameInfo}>
            <span className={styles.gameId}>{session.game_id}</span>
            <span className={styles.moveCount}>Move {session.move_count}</span>
          </div>
        </header>

        <div className={styles.status}>
          <div className={`${styles.statusDot} ${isMyTurn ? styles.dotActive : ''} ${isOver ? styles.dotOver : ''}`} />
          <span
            data-testid="game-status"
            className={`${styles.statusText} ${winnerId === player.id ? styles.win : ''} ${winnerId && winnerId !== player.id ? styles.lose : ''}`}
          >
            {statusText}
          </span>
          {!isSpectator && (() => {
            // Use the first opponent from roomPlayers for the presence indicator.
            // presenceMap may not have an entry yet (defaults to false until
            // the first presence_update WS event arrives).
            const opponentId = roomPlayers.find((p) => p.id !== player.id)?.id ?? null
            if (!opponentId) return null
            const opponentOnline = presenceMap[opponentId] ?? false
            return (
              <span className={styles.opponentPresence} data-testid="opponent-presence">
                <span
                  className={styles.presenceDot}
                  data-online={String(opponentOnline)}
                  data-testid="opponent-presence-dot"
                />
                <span data-testid="opponent-presence-text">
                  {opponentOnline ? 'Opponent online' : 'Opponent offline'}
                </span>
              </span>
            )
          })()}
        </div>

        <div className={styles.boardWrapper}>
          {Renderer ? (
            <Renderer
              gameId={session.game_id}
              gameData={gameData}
              localPlayerId={player.id}
              onMove={(payload) => move.mutate(payload)}
              // Spectators and players whose turn it isn't are both disabled.
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

        {move.error && (
          <p className={styles.error}>
            {move.error instanceof Error ? move.error.message : 'Invalid move'}
          </p>
        )}

        {isOver && (
          <div className={styles.gameOver}>
            <button className="btn btn-ghost" onClick={handleBackToLobby}>
              Back to Lobby
            </button>
            <button className="btn btn-ghost" data-testid="view-replay-btn" onClick={handleViewReplay}>
              View Replay
            </button>
            {/* Spectators do not get a rematch button — only participants vote. */}
            {!isSpectator && (
              <button
                className="btn btn-primary"
                data-testid="rematch-btn"
                onClick={() => rematch.mutate()}
                disabled={votedRematch || rematch.isPending}
              >
                {votedRematch
                  ? `Waiting for opponent… (${rematchVotes}/${totalPlayers})`
                  : rematchVotes > 0
                    ? `Rematch (${rematchVotes}/${totalPlayers} voted)`
                    : 'Rematch'
                }
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