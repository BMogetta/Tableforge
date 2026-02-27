import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAppStore } from '../store'
import { sessions, type GameSession } from '../api'
import { RoomSocket } from '../ws'
import TicTacToeBoard, { type TicTacToeState } from '../components/TicTacToe'
import styles from './Game.module.css'

// GameData is the shape of the state returned by the backend for any game.
interface GameData {
  current_player_id: string
  data: unknown
}

// MoveResult mirrors the backend response for POST /sessions/:id/move.
interface MoveResult {
  session: GameSession
  state: GameData
  is_over: boolean
}

export default function Game() {
  const { sessionId } = useParams<{ sessionId: string }>()
  const player = useAppStore((s) => s.player)!
  const navigate = useNavigate()

  const [session, setSession] = useState<GameSession | null>(null)
  const [gameData, setGameData] = useState<GameData | null>(null)
  const [isOver, setIsOver] = useState(false)
  const [winnerId, setWinnerId] = useState<string | null>(null)
  const [isDraw, setIsDraw] = useState(false)
  const [error, setError] = useState('')
  const [moving, setMoving] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const res = await sessions.get(sessionId!)
      setSession(res.session)
      setGameData(res.state)
    } catch {
      setError('Failed to load game')
    }
  }, [sessionId])

  useEffect(() => {
    refresh()
  }, [refresh])

  // Subscribe to WebSocket events once we know the room ID.
  useEffect(() => {
    if (!session?.room_id) return
    const socket = new RoomSocket(session.room_id)
    socket.connect()

    const off = socket.on((event) => {
      if (event.type === 'move_applied') {
        const payload = event.payload as MoveResult
        setSession(payload.session)
        setGameData(payload.state)
      }
      if (event.type === 'game_over') {
        const payload = event.payload as MoveResult & {
          result?: { winner_id?: string; status?: string }
        }
        setSession(payload.session)
        setGameData(payload.state)
        setIsOver(true)
        if (payload.result?.winner_id) {
          setWinnerId(payload.result.winner_id)
        } else if (payload.result?.status === 'draw') {
          setIsDraw(true)
        }
      }
    })

    return () => { off(); socket.close() }
  }, [session?.room_id])

  async function handleMove(payload: Record<string, unknown>) {
    if (!gameData || moving || isOver) return
    if (gameData.current_player_id !== player.id) return

    setMoving(true)
    setError('')
    try {
      const res = await sessions.move(sessionId!, player.id, payload)
      setSession(res.session)
      setGameData(res.state)
      if (res.is_over) setIsOver(true)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Invalid move')
    } finally {
      setMoving(false)
    }
  }

  if (!session || !gameData) {
    return (
      <div className={styles.centered}>
        <p className="pulse" style={{ color: 'var(--text-muted)', letterSpacing: '0.1em' }}>Loading game...</p>
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
          <span className={`${styles.statusText} ${winnerId === player.id ? styles.win : ''} ${winnerId && winnerId !== player.id ? styles.lose : ''}`}>
            {statusText}
          </span>
        </div>

        <div className={styles.boardWrapper}>
          <GameRenderer
            gameId={session.game_id}
            gameData={gameData}
            localPlayerId={player.id}
            onMove={handleMove}
            disabled={!isMyTurn || moving}
            isOver={isOver}
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

// GameRenderer routes to the correct board component based on game_id.
// Add new games here as they are implemented.
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