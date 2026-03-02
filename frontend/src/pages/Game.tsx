import { useEffect, useRef, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { useAppStore } from '../store'
import { sessions, type GameSession } from '../api'
import { keys } from '../queryClient'
import TicTacToeBoard, { type TicTacToeState } from '../components/TicTacToe'
import styles from './Game.module.css'
import SurrenderModal from '../components/SurrenderModal'

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

export default function Game() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const player = useAppStore((s) => s.player)!
  const socket = useAppStore((s) => s.socket)
  const leaveRoom = useAppStore((s) => s.leaveRoom)
  const navigate = useNavigate()
  const qc = useQueryClient()

  const [isOver, setIsOver] = useState(false)
  const [winnerId, setWinnerId] = useState<string | null>(null)
  const [isDraw, setIsDraw] = useState(false)
  const [showSurrenderModal, setShowSurrenderModal] = useState(false)
  const [rematchVotes, setRematchVotes] = useState(0)
  const [totalPlayers, setTotalPlayers] = useState(0)
  const [votedRematch, setVotedRematch] = useState(false)
  const setPendingRematch = useAppStore((s) => s.setPendingRematch) 

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

  // Sync game-over state from fetch (covers page refresh and missed WS events).
  useEffect(() => {
    if (!data?.session?.finished_at) return
    setIsOver(true)
    const result = (data as any).result as { winner_id?: string; is_draw?: boolean } | null
    if (result?.winner_id) setWinnerId(result.winner_id)
    else if (result?.is_draw) setIsDraw(true)
  }, [data?.session?.finished_at, (data as any)?.result])

  // Subscribe to the global socket for real-time game events.
  // The socket was opened in Room.tsx and survives the Room→Game navigation.
  useEffect((): (() => void) | void => {
    if (!socket) return

    const off = socket.on((event) => {
      // On reconnect, refetch to catch any events missed during the gap.
      if (event.type === 'ws_connected') {
        qc.invalidateQueries({ queryKey: keys.session(sessionId!) })
      }
      if (event.type === 'move_applied') {
        const payload = event.payload as MoveResult
        qc.setQueryData(keys.session(sessionId!), {
          session: payload.session,
          state: payload.state,
          is_over: false,
        })
      }
      if (event.type === 'game_over') {
        const payload = event.payload as MoveResult
        qc.setQueryData(keys.session(sessionId!), {
          session: payload.session,
          state: payload.state,
          is_over: true,
        })
        setIsOver(true)
        if (payload.result?.winner_id) setWinnerId(payload.result.winner_id)
        else if (payload.result?.is_draw) setIsDraw(true)
      }
      if (event.type === 'rematch_vote') {
        const payload = event.payload as { votes: number; total_players: number }
        setRematchVotes(payload.votes)
        setTotalPlayers(payload.total_players)
      }
      if (event.type === 'rematch_started') {
        const payload = event.payload as { session: { id: string } }
        setPendingRematch(payload.session.id)
      }
    })

    return () => off() // Unsubscribe only — do not close the socket here.
  }, [socket, sessionId, qc])

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
      // If both players voted, rematch_started WS event handles navigation.
    },
  })

  function handleBackToLobby() {
    leaveRoom()
    navigate('/')
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

  const isMyTurn = gameData.current_player_id === player.id && !isOver

  let statusText = ''
  if (winnerId) {
    statusText = winnerId === player.id ? 'You won!' : 'You lost.'
  } else if (isDraw) {
    statusText = 'Draw!'
  } else if (isMyTurn) {
    statusText = 'Your turn'
  } else {
    statusText = "Opponent's turn"
  }

  return (
    <div className={`${styles.root} page-enter`}>
      <div className={styles.panel}>
        <header className={styles.header}>
          <button
            className="btn btn-ghost"
            onClick={() => isOver ? handleBackToLobby() : setShowSurrenderModal(true)}
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
        </div>

        <div className={styles.boardWrapper}>
          <GameRenderer
            gameId={session.game_id}
            gameData={gameData}
            localPlayerId={player.id}
            onMove={(payload) => move.mutate(payload)}
            disabled={!isMyTurn || move.isPending}
            isOver={isOver}
          />
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
            <button
              className="btn btn-primary"
              data-testid="rematch-btn"
              onClick={() => rematch.mutate()}
              disabled={votedRematch || rematch.isPending}
            >
              {votedRematch
                ? totalPlayers > 0
                  ? `Waiting for opponent… (${rematchVotes}/${totalPlayers})`
                  : 'Waiting for opponent…'
                : 'Rematch'}
            </button>
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

function GameRenderer({
  gameId,
  gameData,
  localPlayerId,
  onMove,
  disabled,
}: {
  gameId: string
  gameData: GameData
  localPlayerId: string
  onMove: (payload: Record<string, unknown>) => void
  disabled: boolean
  isOver: boolean
}) {
  switch (gameId) {
    case 'tictactoe':
      return (
        <TicTacToeBoard
          state={gameData.data as TicTacToeState}
          currentPlayerId={gameData.current_player_id}
          localPlayerId={localPlayerId}
          onMove={(cell) => onMove({ cell })}
          disabled={disabled}
        />
      )
    default:
      return (
        <p style={{ color: 'var(--text-muted)', fontSize: 13 }}>
          No renderer available for game: {gameId}
        </p>
      )
  }
}