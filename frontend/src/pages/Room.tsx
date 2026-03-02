import { useEffect, useState, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { useAppStore } from '../store'
import { rooms, type RoomView } from '../api'
import styles from './Room.module.css'

type SocketStatus = 'connecting' | 'connected' | 'reconnecting' | 'disconnected'

export default function Room() {
  const { roomId } = useParams<{ roomId: string }>()
  const player = useAppStore((s) => s.player)!
  const joinRoom = useAppStore((s) => s.joinRoom)
  const socket = useAppStore((s) => s.socket)
  const leaveRoom = useAppStore((s) => s.leaveRoom)
  const navigate = useNavigate()

  const [view, setView] = useState<RoomView | null>(null)
  const [error, setError] = useState('')
  const [starting, setStarting] = useState(false)
  const [socketStatus, setSocketStatus] = useState<SocketStatus>('connecting')

  const refresh = useCallback(() => {
    rooms.get(roomId!).then(setView).catch(() => setError('Room not found'))
  }, [roomId])

  // Connect the global socket when entering the room.
  // Do NOT close on unmount — Game.tsx reuses the same socket instance.
  useEffect(() => {
    joinRoom(roomId!)
  }, [roomId, joinRoom])

  // Subscribe to socket events for real-time room state updates.
  useEffect(() => {
    if (!socket) return
    refresh()

    const off = socket.on((event) => {
      if (event.type === 'ws_connected')    setSocketStatus('connected')
      if (event.type === 'ws_reconnecting') setSocketStatus('reconnecting')
      if (event.type === 'ws_disconnected') setSocketStatus('disconnected')

      if (event.type === 'player_joined' || event.type === 'player_left') {
        refresh()
      }
      if (event.type === 'game_started') {
        const payload = event.payload as { session: { id: string } }
        if (!starting) {
          // Non-owners navigate via WS event; the owner navigates via HTTP response.
          navigate(`/game/${payload.session.id}`)
        }
      }
    })

    return () => off() // Unsubscribe only — do not close the socket here.
  }, [socket, refresh, navigate, starting])

  async function handleStart() {
    setStarting(true)
    try {
      const session = await rooms.start(roomId!, player.id)
      navigate(`/game/${session.id}`)
    } catch (e: unknown) {
      setError(e instanceof Error ? e.message : 'Failed to start game')
      setStarting(false)
    }
  }

  async function handleLeave() {
    await rooms.leave(roomId!, player.id).catch(() => {})
    leaveRoom()
    navigate('/')
  }

  if (!view) return <LoadingScreen />

  const { room } = view
  const isOwner = room.owner_id === player.id
  const canStart = isOwner && view.players.length >= 2

  return (
    <div className={`${styles.root} page-enter`}>
      <ConnectionBanner status={socketStatus} />

      <div className={styles.panel}>
        <header className={styles.header}>
          <div>
            <p className={styles.gameLabel}>{room.game_id}</p>
            <h1 data-testid="room-code" className={styles.code}>{room.code}</h1>
          </div>
          <span className="badge badge-amber">Waiting</span>
        </header>

        <hr className="divider" />

        <section className={styles.playersSection}>
          <p className="label" data-testid="player-count">
            Players ({view.players.length}/{room.max_players})
          </p>
          <div className={styles.playerList}>
            {view.players.map((p) => (
              <div key={p.id} className={styles.playerRow}>
                {p.avatar_url && <img src={p.avatar_url} alt="" className={styles.avatar} />}
                <span className={styles.playerName}>{p.username}</span>
                {p.id === room.owner_id && <span className="badge badge-amber">Host</span>}
                {p.id === player.id && <span className="badge badge-muted">You</span>}
              </div>
            ))}
            {Array.from({ length: room.max_players - view.players.length }).map((_, i) => (
              <div key={i} className={`${styles.playerRow} ${styles.empty}`}>
                <div className={styles.emptySlot} />
                <span className={styles.waitingText}>Waiting for player...</span>
              </div>
            ))}
          </div>
        </section>

        <hr className="divider" />

        <div className={styles.shareSection}>
          <p className="label">Invite Code</p>
          <div className={styles.codeBox}>
            <span className={styles.codeDisplay}>{room.code}</span>
            <button
              className="btn btn-ghost"
              style={{ padding: '4px 10px', fontSize: 11 }}
              onClick={() => navigator.clipboard.writeText(room.code)}
            >
              Copy
            </button>
          </div>
        </div>

        <hr className="divider" />

        <div className={styles.actions}>
          {isOwner ? (
            <button
              data-testid="start-game-btn"
              data-can-start={canStart}
              className="btn btn-primary"
              onClick={handleStart}
              disabled={!canStart || starting}
              style={{ flex: 1 }}
            >
              {starting ? 'Starting...' : canStart ? 'Start Game' : `Need ${2 - view.players.length} more player(s)`}
            </button>
          ) : (
            <p className={styles.waitingHost}>Waiting for host to start the game...</p>
          )}
          <button className="btn btn-danger" onClick={handleLeave}>Leave</button>
        </div>

        {error && <p className={styles.error}>{error}</p>}
      </div>
    </div>
  )
}

// --- Connection banner -------------------------------------------------------

function ConnectionBanner({ status }: { status: SocketStatus }) {
  if (status === 'connected') return null

  const config: Record<Exclude<SocketStatus, 'connected'>, { text: string; className: string }> = {
    connecting:   { text: 'Connecting...',                          className: styles.bannerConnecting },
    reconnecting: { text: 'Connection lost — reconnecting...',      className: styles.bannerReconnecting },
    disconnected: { text: 'Disconnected. Please refresh the page.', className: styles.bannerDisconnected },
  }

  const { text, className } = config[status]

  return (
    <div className={`${styles.banner} ${className}`}>
      {status !== 'disconnected' && <span className={styles.bannerDot} />}
      {text}
    </div>
  )
}

// --- Loading screen ----------------------------------------------------------

function LoadingScreen() {
  return (
    <div style={{ display: 'flex', alignItems: 'center', justifyContent: 'center', height: '100vh' }}>
      <p className="pulse" style={{ color: 'var(--text-muted)', letterSpacing: '0.1em' }}>Loading room...</p>
    </div>
  )
}