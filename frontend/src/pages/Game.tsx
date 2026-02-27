import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAppStore } from '../store'
import { sessions, type GameSession } from '../api'
import { RoomSocket } from '../ws'
import styles from './Game.module.css'

interface TicTacToeState {
  board: (string | null)[]
  current_player_id: string
  winner_id?: string
  is_draw?: boolean
}

interface MoveResult {
  session: GameSession
  state: { current_player_id: string; data: TicTacToeState }
  is_over: boolean
}

export default function Game() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const player = useAppStore((s) => s.player)!
  const navigate = useNavigate()

  const [session, setSession] = useState<GameSession | null>(null)
  const [state, setState] = useState<TicTacToeState | null>(null)
  const [isOver, setIsOver] = useState(false)
  const [error, setError] = useState('')
  const [moving, setMoving] = useState(false)
  const [roomId, setRoomId] = useState<string | null>(null)

  const refresh = useCallback(async () => {
    try {
      const res = await sessions.get(sessionId!)
      setSession(res.session)
      setRoomId(res.session.room_id)
      const data = res.state as { current_player_id: string; data: TicTacToeState }
      setState(data.data)
    } catch {
      setError('Failed to load game')
    }
  }, [sessionId])

  useEffect(() => {
    refresh()
  }, [refresh])

  useEffect(() => {
    if (!roomId) return
    const socket = new RoomSocket(roomId)
    socket.connect()
    const off = socket.on((event) => {
      if (event.type === 'move_applied' || event.type === 'game_over') {
        const payload = event.payload as MoveResult
        setSession(payload.session)
        setState(payload.state.data as TicTacToeState)
        if (event.type === 'game_over') setIsOver(true)
      }
    })
    return () => { off(); socket.close() }
  }, [roomId])

  async function handleMove(position: number) {
    if (!state || moving || isOver) return
    if (state.current_player_id !== player.id) return
    if (state.board[position] !== null) return

    setMoving(true)
    setError('')
    try {
      const res = await sessions.move(sessionId!, player.id, { position })
      setSession(res.session)
      const gameState = res.state.data as TicTacToeState
      setState(gameState)
      if (res.is_over) setIsOver(true)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Invalid move')
    } finally {
      setMoving(false)
    }
  }

  if (!session || !state) {
    return (
      <div className={styles.centered}>
        <p className="pulse" style={{ color: 'var(--text-muted)', letterSpacing: '0.1em' }}>Loading game...</p>
      </div>
    )
  }

  const isMyTurn = state.current_player_id === player.id && !isOver
  const winner = state.winner_id
  const isDraw = state.is_draw

  let statusText = ''
  if (winner) {
    statusText = winner === player.id ? 'You won!' : 'You lost.'
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
          <button className="btn btn-ghost" onClick={() => navigate('/')} style={{ padding: '4px 10px', fontSize: 11 }}>
            ← Lobby
          </button>
          <div className={styles.gameInfo}>
            <span className={styles.gameId}>{session.game_id}</span>
            <span className={styles.moveCount}>Move {session.move_count}</span>
          </div>
        </header>

        <div className={styles.status}>
          <div className={`${styles.statusDot} ${isMyTurn ? styles.dotActive : ''} ${isOver ? styles.dotOver : ''}`} />
          <span className={`${styles.statusText} ${winner === player.id ? styles.win : ''} ${winner && winner !== player.id ? styles.lose : ''}`}>
            {statusText}
          </span>
        </div>

        <div className={styles.boardWrapper}>
          <TicTacToeBoard
            board={state.board}
            playerId={player.id}
            currentPlayerId={state.current_player_id}
            onMove={handleMove}
            disabled={!isMyTurn || moving}
          />
        </div>

        {error && <p className={styles.error}>{error}</p>}

        {isOver && (
          <div className={styles.gameOver}>
            <button className="btn btn-primary" onClick={() => navigate('/')}>
              Back to Lobby
            </button>
          </div>
        )}
      </div>
    </div>
  )
}

function TicTacToeBoard({
  board,
  playerId,
  currentPlayerId,
  onMove,
  disabled,
}: {
  board: (string | null)[]
  playerId: string
  currentPlayerId: string
  onMove: (i: number) => void
  disabled: boolean
}) {
  // Determine symbol for current player (X if first to move, O if second)
  // We use a simple heuristic: count non-null cells
  const xCount = board.filter(Boolean).length
  // Player is X if it's their first move opportunity (they go first)
  // We'll just display X/O based on which cells are theirs vs opponent's
  // Since we don't have player assignment in state, use position parity
  const getSymbol = (cellPlayerId: string) =>
    cellPlayerId === playerId ? 'X' : 'O'

  return (
    <div className={styles.board}>
      {board.map((cell, i) => (
        <button
          key={i}
          className={`${styles.cell} ${cell ? styles.cellFilled : ''} ${!cell && !disabled ? styles.cellHoverable : ''}`}
          onClick={() => onMove(i)}
          disabled={disabled || !!cell}
        >
          {cell && (
            <span className={`${styles.symbol} ${getSymbol(cell) === 'X' ? styles.symbolX : styles.symbolO}`}>
              {getSymbol(cell)}
            </span>
          )}
          {!cell && !disabled && <span className={styles.hoverSymbol}>·</span>}
        </button>
      ))}
      {/* Grid lines */}
      <div className={styles.gridLines}>
        <div className={styles.vLine} style={{ left: '33.33%' }} />
        <div className={styles.vLine} style={{ left: '66.66%' }} />
        <div className={styles.hLine} style={{ top: '33.33%' }} />
        <div className={styles.hLine} style={{ top: '66.66%' }} />
      </div>
    </div>
  )
}
